export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig()
  const path = event.path.replace(/^\/api/, '')
  const backendUrl = `${config.backendUrl}/api${path}`

  const method = event.method
  const body = method !== 'GET' && method !== 'HEAD' ? await readRawBody(event) : undefined

  const headers: Record<string, string> = {}
  const incomingHeaders = getRequestHeaders(event)
  for (const [key, value] of Object.entries(incomingHeaders)) {
    if (value && !['host', 'connection'].includes(key.toLowerCase())) {
      headers[key] = value
    }
  }

  const upstream = await fetch(backendUrl, { method, headers, body })

  const contentType = upstream.headers.get('content-type') ?? ''

  // SSE：直接 pipe stream，不緩衝
  if (contentType.includes('text/event-stream')) {
    setResponseHeader(event, 'Content-Type', 'text/event-stream')
    setResponseHeader(event, 'Cache-Control', 'no-cache')
    setResponseHeader(event, 'Connection', 'keep-alive')
    setResponseHeader(event, 'X-Accel-Buffering', 'no')
    setResponseStatus(event, upstream.status)
    return sendStream(event, upstream.body!)
  }

  // 一般 JSON 回應
  setResponseStatus(event, upstream.status)
  upstream.headers.forEach((value, key) => {
    if (!['transfer-encoding'].includes(key.toLowerCase())) {
      setResponseHeader(event, key, value)
    }
  })
  return upstream.body
})
