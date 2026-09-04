/* No Lights map — vanilla Mapbox GL, no framework. */
(function () {
  const { mapboxToken, radiusKM } = window.NOLIGHTS

  if (!mapboxToken) {
    document.getElementById('status').textContent = 'Falta MAPBOX_TOKEN'
    return
  }

  mapboxgl.accessToken = mapboxToken

  const map = new mapboxgl.Map({
    container: 'map',
    style: 'mapbox://styles/mapbox/dark-v11',
    center: [-66.9, 10.5],
    zoom: 12,
  })

  const status = document.getElementById('status')
  const markers = new Map()

  async function refresh(lon, lat) {
    const url = `/api/consultar-radio?lon=${lon}&lat=${lat}`
    const res = await fetch(url)
    const data = await res.json()
    status.textContent = `${data.count} corte(s) en ${data.radio_km} km`

    for (const p of data.points) {
      if (markers.has(p.usuario_id)) continue
      const el = document.createElement('div')
      el.className = 'marker'
      const m = new mapboxgl.Marker(el)
        .setLngLat([p.longitud, p.latitud])
        .addTo(map)
      markers.set(p.usuario_id, m)
    }
  }

  function track() {
    navigator.geolocation.getCurrentPosition(
      (pos) => refresh(pos.coords.longitude, pos.coords.latitude),
      () => refresh(-66.9, 10.5),
    )
  }

  map.addControl(new mapboxgl.NavigationControl(), 'top-right')
  map.addControl(new mapboxgl.GeolocateControl(), 'top-right')

  map.on('load', track)

  document.getElementById('report-btn').addEventListener('click', () => {
    const center = map.getCenter()
    fetch('/api/reportar', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        usuario_id: 'web-user',
        longitud: center.lng,
        latitud: center.lat,
        tiene_luz: false,
      }),
    }).then(() => track())
  })
})()
