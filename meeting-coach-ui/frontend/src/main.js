import { EventsOn, Invoke } from '@wailsapp/runtime'

function appendMeetingLine(text) {
  const el = document.getElementById('meeting-console')
  if (!el) return
  const p = document.createElement('div')
  p.textContent = text
  el.appendChild(p)
  el.scrollTop = el.scrollHeight
}

function setMeetingState(text) {
  const el = document.getElementById('meeting-state')
  el.textContent = text
}

function formatDurationHuman(msOrNs) {
  if (!msOrNs) return '0s'
  let s = 0
  if (typeof msOrNs === 'number') {
    // guess: if > 1e9, it's ns
    if (msOrNs > 1e9) s = Math.floor(msOrNs / 1e9)
    else s = Math.floor(msOrNs / 1000)
  } else {
    return String(msOrNs)
  }
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h) return `${h}h ${m}m ${sec}s`
  if (m) return `${m}m ${sec}s`
  return `${sec}s`
}

function renderActivity(entries, summary) {
  const tbody = document.getElementById('activity-table-body')
  tbody.innerHTML = ''
  (entries || []).slice().reverse().forEach(e => {
    const tr = document.createElement('tr')
    const timeStr = e.start_time || e.StartTime || ''
    const start = timeStr ? new Date(timeStr).toLocaleTimeString() : ''
    const app = e.app || e.App || ''
    const url = e.url || e.URL || '-'
    const dur = e.duration || e.Duration || 0
    tr.innerHTML = `<td>${start}</td><td>${app}</td><td>${url}</td><td>${formatDurationHuman(dur)}</td>`
    tbody.appendChild(tr)
  })

  const sumDiv = document.getElementById('summary')
  sumDiv.innerHTML = ''
  const s = summary || {}
  Object.keys(s).forEach(k => {
    const el = document.createElement('div')
    el.textContent = `${k}: ${formatDurationHuman(s[k] || 0)}`
    sumDiv.appendChild(el)
  })
}

// Listen to raw detector log lines (we did NOT change detector)
EventsOn('detector:log', payload => {
  const line = payload.line || payload
  appendMeetingLine(line)

  // simple parsing to set meeting state
  if (line.includes("📱 Meeting app detected:")) {
    // "📱 Meeting app detected: Google Chrome (https://meet.google.com/...)"
    const parts = line.split(":")
    if (parts.length >= 2) {
      const app = parts.slice(1).join(":").trim()
      setMeetingState("DETECTING — " + app)
    }
  } else if (line.includes("MEETING STARTED") || line.includes("MEETING STARTED!")) {
    setMeetingState("🟢 MEETING ACTIVE")
  } else if (line.includes("MEETING ENDED")) {
    setMeetingState("🔴 MEETING ENDED")
  } else if (line.toLowerCase().includes("screenpipe unreachable")) {
    setMeetingState("⚠️ Screenpipe unreachable")
  }
})

// Listen to activity snapshots
EventsOn('activity:update', payload => {
  renderActivity(payload.entries || [], payload.summary || {})
})

// start backend (calls App.Start)
Invoke('App.Start').catch(e => {
  appendMeetingLine("Error starting backend: " + String(e))
})

// update time in header
setInterval(() => {
  const t = document.getElementById('time')
  if (t) t.textContent = new Date().toLocaleString()
}, 1000)