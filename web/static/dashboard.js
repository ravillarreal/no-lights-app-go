/* No Lights dashboard — server-rendered HTML + fetched JSON. */
(async function () {
  async function json(path) {
    const res = await fetch(path)
    return res.json()
  }

  const summary = await json('/api/stats/summary')
  const kpis = document.getElementById('kpis')
  kpis.innerHTML = [
    ['Cortes completados', summary.total_completed],
    ['Cortes activos', summary.active_outages],
    ['Duración media (min)', summary.avg_duration_min ?? '—'],
    ['Zonas afectadas', summary.affected_zones],
  ].map(([label, value]) => `
    <div class="kpi"><div class="value">${value}</div><div class="label">${label}</div></div>
  `).join('')

  const byDay = await json('/api/stats/by-day')
  document.getElementById('by-day').innerHTML = '<h3>Cortes por día</h3>' + byDay.map(d =>
    `<div>${d.date}: <b>${d.outages}</b> (avg ${d.avg_duration_min} min)</div>`
  ).join('')

  const byZone = await json('/api/stats/by-zone?field=neighborhood')
  document.getElementById('by-zone').innerHTML = '<h3>Zonas más afectadas</h3>' + byZone.map(z =>
    `<div>${z.zone}: <b>${z.outages}</b></div>`
  ).join('')
})()
