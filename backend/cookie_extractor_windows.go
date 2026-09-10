//go:build windows && !cli

package backend

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
	_ "modernc.org/sqlite"
)

var (
	modCrypt32            = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = modCrypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func cryptUnprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	var inBlob, outBlob dataBlob
	inBlob.cbData = uint32(len(data))
	inBlob.pbData = &data[0]

	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, err
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(outBlob.pbData)))

	out := make([]byte, outBlob.cbData)
	copy(out, unsafe.Slice(outBlob.pbData, outBlob.cbData))
	return out, nil
}

func getMasterKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}
	var state struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	rawKey, err := base64.StdEncoding.DecodeString(state.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, err
	}
	if len(rawKey) < 5 || string(rawKey[:5]) != "DPAPI" {
		return nil, fmt.Errorf("invalid DPAPI prefix")
	}
	return cryptUnprotectData(rawKey[5:])
}

func decryptCookie(encrypted []byte, key []byte) (string, error) {
	if len(encrypted) < 3 {
		return "", fmt.Errorf("encrypted value too short")
	}
	prefix := string(encrypted[:3])
	if prefix != "v10" && prefix != "v11" {
		plain, err := cryptUnprotectData(encrypted)
		if err == nil {
			return string(plain), nil
		}
		return "", fmt.Errorf("unknown prefix %s", prefix)
	}
	if len(encrypted) < 3+12+16 {
		return "", fmt.Errorf("payload too short for AES-GCM")
	}
	nonce := encrypted[3 : 3+12]
	ciphertext := encrypted[3+12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func findEBWebViewDir() string {
	appData := os.Getenv("AppData")
	candidates := []string{
		filepath.Join(appData, filepath.Base(os.Args[0]), "EBWebView"),
		filepath.Join(appData, "gotohp.exe", "EBWebView"),
		filepath.Join(appData, "gotohp_debug.exe", "EBWebView"),
		filepath.Join(appData, "gotohp", "EBWebView"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

func extractOAuthTokenFromSQLite() (string, error) {
	baseDir := findEBWebViewDir()
	if baseDir == "" {
		return "", errors.New("EBWebView directory not found")
	}

	localStatePath := filepath.Join(baseDir, "Local State")
	cookiePath := filepath.Join(baseDir, "Default", "Network", "Cookies")

	if _, err := os.Stat(cookiePath); err != nil {
		return "", nil // Cookie file not created yet
	}

	key, err := getMasterKey(localStatePath)
	if err != nil {
		return "", fmt.Errorf("getMasterKey: %w", err)
	}

	dbURI := fmt.Sprintf("file:%s?mode=ro&immutable=1", filepath.ToSlash(cookiePath))
	db, err := sql.Open("sqlite", dbURI)
	if err != nil {
		return "", fmt.Errorf("sql.Open: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT value, encrypted_value 
		FROM cookies 
		WHERE (name = 'oauth_token' OR name LIKE '%oauth%')
		  AND (host_key LIKE '%google%' OR host_key LIKE '%.google.com')
		ORDER BY creation_utc DESC
		LIMIT 5
	`)
	if err != nil {
		return "", nil // Table may be temporarily unavailable or locked
	}
	defer rows.Close()

	for rows.Next() {
		var plainVal string
		var encVal []byte
		if err := rows.Scan(&plainVal, &encVal); err != nil {
			continue
		}
		if len(plainVal) >= 16 {
			return plainVal, nil
		}
		if len(encVal) > 0 {
			decrypted, err := decryptCookie(encVal, key)
			if err == nil && len(decrypted) >= 16 {
				return decrypted, nil
			}
		}
	}

	return "", nil
}

// COM structures for ICoreWebView2CookieManager
type iCoreWebView2CookieListVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	GetCount       uintptr
	GetItem        uintptr
}

type iCoreWebView2CookieList struct {
	vtbl *iCoreWebView2CookieListVtbl
}

type iCoreWebView2CookieVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	GetName        uintptr
	GetValue       uintptr
}

type iCoreWebView2Cookie struct {
	vtbl *iCoreWebView2CookieVtbl
}

type cookieHandlerVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

type cookieCompletedHandler struct {
	vtbl      *cookieHandlerVtbl
	tokenChan chan string
}

var (
	cookieHandlerMap        sync.Map
	cookieHandlerOnce       sync.Once
	globalCookieHandlerVtbl cookieHandlerVtbl
)

func getCookieDetails(cPtr uintptr) (name, val string) {
	c := (*iCoreWebView2Cookie)(unsafe.Pointer(cPtr))
	var pName, pVal *uint16
	syscall.SyscallN(c.vtbl.GetName, cPtr, uintptr(unsafe.Pointer(&pName)))
	syscall.SyscallN(c.vtbl.GetValue, cPtr, uintptr(unsafe.Pointer(&pVal)))
	if pName != nil {
		name = windows.UTF16PtrToString(pName)
		windows.CoTaskMemFree(unsafe.Pointer(pName))
	}
	if pVal != nil {
		val = windows.UTF16PtrToString(pVal)
		windows.CoTaskMemFree(unsafe.Pointer(pVal))
	}
	return
}

func initCookieHandlerVtbl() {
	cookieHandlerOnce.Do(func() {
		globalCookieHandlerVtbl = cookieHandlerVtbl{
			QueryInterface: windows.NewCallback(func(this, riid, ppvObject uintptr) uintptr {
				if ppvObject == 0 {
					return 0x80070057 // E_POINTER
				}
				*(*uintptr)(unsafe.Pointer(ppvObject)) = this
				return 0 // S_OK
			}),
			AddRef: windows.NewCallback(func(this uintptr) uintptr {
				return 2
			}),
			Release: windows.NewCallback(func(this uintptr) uintptr {
				return 1
			}),
			Invoke: windows.NewCallback(func(this, errorCode, cookieList uintptr) uintptr {
				val, ok := cookieHandlerMap.LoadAndDelete(this)
				var tokenChan chan string
				if ok && val != nil {
					tokenChan = val.(*cookieCompletedHandler).tokenChan
				}

				var capturedToken string
				if errorCode == 0 && cookieList != 0 {
					list := (*iCoreWebView2CookieList)(unsafe.Pointer(cookieList))
					var count uint32
					syscall.SyscallN(list.vtbl.GetCount, cookieList, uintptr(unsafe.Pointer(&count)))
					for i := uint32(0); i < count; i++ {
						var cPtr uintptr
						syscall.SyscallN(list.vtbl.GetItem, cookieList, uintptr(i), uintptr(unsafe.Pointer(&cPtr)))
						if cPtr != 0 {
							name, val := getCookieDetails(cPtr)
							if (name == "oauth_token" || strings.Contains(name, "oauth") || strings.HasPrefix(val, "oauth2_4/")) && len(val) >= 16 {
								log.Printf("[InAppAuth] Found OAuth cookie in WebView2: name=%s, len=%d\n", name, len(val))
								capturedToken = val
							}
						}
					}
				}

				if tokenChan != nil {
					select {
					case tokenChan <- capturedToken:
					default:
					}
				}
				return 0
			}),
		}
	})
}

func getCookieManagerPtr(win *application.WebviewWindow) (uintptr, uintptr, error) {
	if win == nil {
		return 0, 0, errors.New("win is nil")
	}
	winVal := reflect.ValueOf(win)
	if winVal.Kind() == reflect.Pointer {
		winVal = winVal.Elem()
	}
	implField := winVal.FieldByName("impl")
	if !implField.IsValid() {
		return 0, 0, errors.New("implField invalid")
	}
	implInterface := reflect.NewAt(implField.Type(), unsafe.Pointer(implField.UnsafeAddr())).Elem()
	if implInterface.IsNil() {
		return 0, 0, errors.New("implInterface nil")
	}
	implPtr := implInterface.Elem()
	implStruct := implPtr.Elem()
	chromField := implStruct.FieldByName("chromium")
	if !chromField.IsValid() {
		return 0, 0, errors.New("chromField invalid")
	}
	chromPtr := reflect.NewAt(chromField.Type(), unsafe.Pointer(chromField.UnsafeAddr())).Elem()
	if chromPtr.IsNil() {
		return 0, 0, errors.New("chromPtr nil")
	}

	cmMethod := chromPtr.MethodByName("GetCookieManager")
	if !cmMethod.IsValid() {
		return 0, 0, errors.New("GetCookieManager method not found")
	}
	cmRes := cmMethod.Call(nil)
	if len(cmRes) < 2 || !cmRes[1].IsNil() {
		return 0, 0, errors.New("GetCookieManager call failed")
	}
	cm := cmRes[0]
	if cm.IsNil() {
		return 0, 0, errors.New("cookieManager is nil")
	}

	cmVal := cm.Pointer()
	cmVtbl := *(**uintptr)(unsafe.Pointer(cmVal))
	getCookiesProc := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(cmVtbl)) + 5*unsafe.Sizeof(uintptr(0))))
	return cmVal, getCookiesProc, nil
}

func extractOAuthTokenFromWebView2(win *application.WebviewWindow) string {
	if win == nil {
		return ""
	}
	initCookieHandlerVtbl()

	tokenChan := make(chan string, 1)
	h := &cookieCompletedHandler{
		vtbl:      &globalCookieHandlerVtbl,
		tokenChan: tokenChan,
	}
	thisPtr := uintptr(unsafe.Pointer(h))
	cookieHandlerMap.Store(thisPtr, h)

	application.InvokeAsync(func() {
		cmVal, getCookiesProc, err := getCookieManagerPtr(win)
		if err != nil {
			cookieHandlerMap.Delete(thisPtr)
			tokenChan <- ""
			return
		}
		uriPtr, _ := windows.UTF16PtrFromString("")
		syscall.SyscallN(
			getCookiesProc,
			cmVal,
			uintptr(unsafe.Pointer(uriPtr)),
			thisPtr,
		)
	})

	select {
	case t := <-tokenChan:
		return t
	case <-time.After(1200 * time.Millisecond):
		cookieHandlerMap.Delete(thisPtr)
		return ""
	}
}

// extractOAuthTokenFromWindow retrieves oauth_token from the live WebView2 session
// or falls back to the profile database on disk.
func extractOAuthTokenFromWindow(win *application.WebviewWindow) (string, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[InAppAuth] Panic recovered in extractOAuthTokenFromWindow: %v\n", r)
		}
	}()

	// 1. First try live WebView2 in-memory cookies (captures session cookies like oauth_token immediately)
	if win != nil {
		if token := extractOAuthTokenFromWebView2(win); len(token) >= 16 {
			return token, nil
		}
	}

	// 2. Fallback to SQLite database on disk
	return extractOAuthTokenFromSQLite()
}
