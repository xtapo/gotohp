export interface UploadSuccess {
  path: string
  mediaKey: string
}

export interface SkippedUploadResult {
  paths: string[]
  code: string
  reason: string
}

export interface FailedUploadItem {
  path: string
  fileName: string
  error: string
}

export interface UploadResults {
  success: UploadSuccess[]
  fail: string[]
  failedItems: FailedUploadItem[]
  skipped: SkippedUploadResult[]
  warnings: SkippedUploadResult[]
}

export interface UploadResultEvent {
  MediaKey: string
  IsError: boolean
  Skipped: boolean
  ErrorMessage: string
  SkipCode: string
  SkipReason: string
  Path: string
  Paths: string[]
}

export function recordUploadResult(results: UploadResults, event: UploadResultEvent): number {
  if (!results.failedItems) {
    results.failedItems = []
  }

  if (event.IsError) {
    const errorMsg = event.ErrorMessage || 'Unknown upload error'
    results.fail.push(event.ErrorMessage ? `${event.Path}: ${event.ErrorMessage}` : event.Path)
    const fileName = event.Path ? event.Path.split(/[\\/]/).pop() || event.Path : 'Unknown file'
    results.failedItems.push({
      path: event.Path,
      fileName,
      error: errorMsg,
    })
    return 1
  }

  if (event.Skipped) {
    const paths = event.Paths?.length ? event.Paths : [event.Path]
    results.skipped.push({
      paths: paths.filter(Boolean),
      code: event.SkipCode || 'skipped',
      reason: event.SkipReason || 'Skipped by upload policy',
    })
    return 1
  }

  results.success.push({ path: event.Path, mediaKey: event.MediaKey })
  return 1
}
