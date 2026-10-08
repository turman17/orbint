// Cesium viewer setup: a dark, physically lit Earth with a day/night
// terminator, city lights on the night side, atmosphere, stars and a soft
// bloom. Imagery streams keylessly from NASA GIBS and falls back to the
// Natural Earth II tiles bundled with Cesium when the network is unavailable.

import {
  Cartesian3,
  Color,
  EasingFunction,
  GeographicTilingScheme,
  ImageryLayer,
  Ion,
  TileMapServiceImageryProvider,
  Viewer,
  WebMapServiceImageryProvider,
  buildModuleUrl,
} from 'cesium'

// No Cesium Ion assets are used; silence the default-token warning.
Ion.defaultAccessToken = ''

/** Tunable look of the scene. Keep changes here, not scattered in code. */
export const LOOK = {
  /** Globe colour shown before imagery tiles arrive. */
  baseColor: '#07111f',
  /** Overall sun intensity on the ground atmosphere (Cesium default 10). */
  atmosphereLightIntensity: 12,
  /** Camera distances (m) where day/night lighting stops / resumes. */
  lightingFadeOutDistance: 1.0e6,
  lightingFadeInDistance: 2.5e6,
  /** Bloom: a subtle halo on bright pixels. Only on at the 'high' quality tier. */
  bloom: { contrast: 118, brightness: -0.38, delta: 1.0, sigma: 2.6, stepSize: 1.0 },
  /** Camera limits (m from the ellipsoid surface). */
  minimumZoomDistance: 1.5e5,
  maximumZoomDistance: 2.0e8,
  /** Opening shot: start far out, settle to a full-disc view. */
  introStartHeight: 7.5e7,
  introEndHeight: 2.75e7,
  introSeconds: 3.4,
}

/**
 * NASA GIBS, WMS flavour. The WMTS endpoint uses a 2.5·2^n tile matrix that
 * Cesium's geographic scheme cannot address, whereas WMS renders any bounding
 * box, so every Cesium tile maps cleanly. 512 px tiles halve the request count.
 */
const GIBS_WMS = 'https://gibs.earthdata.nasa.gov/wms/epsg4326/best/wms.cgi'

function gibsProvider(layer: string, credit: string): WebMapServiceImageryProvider {
  return new WebMapServiceImageryProvider({
    url: GIBS_WMS,
    layers: layer,
    parameters: { version: '1.1.1', format: 'image/jpeg', transparent: false },
    tilingScheme: new GeographicTilingScheme(),
    tileWidth: 512,
    tileHeight: 512,
    maximumLevel: 7,
    enablePickFeatures: false,
    credit,
  })
}

export function createViewer(container: HTMLElement, creditContainer: HTMLElement): Viewer {
  const viewer = new Viewer(container, {
    animation: false,
    timeline: false,
    baseLayerPicker: false,
    geocoder: false,
    homeButton: false,
    sceneModePicker: false,
    navigationHelpButton: false,
    fullscreenButton: false,
    infoBox: false,
    selectionIndicator: false,
    baseLayer: false,
    shouldAnimate: true,
    creditContainer,
    contextOptions: {
      webgl: { antialias: true, powerPreference: 'high-performance' },
    },
  })

  const { scene } = viewer
  const { globe } = scene

  // Imagery: Blue Marble by day, VIIRS city lights by night. The night layer
  // is fully transparent on the lit side and opaque in shadow, so the two
  // blend along the terminator that Cesium computes from the real sun.
  const day = new ImageryLayer(
    gibsProvider('BlueMarble_ShadedRelief_Bathymetry', 'NASA GIBS · Blue Marble'),
  )
  const night = new ImageryLayer(
    gibsProvider('VIIRS_CityLights_2012', 'NASA GIBS · VIIRS city lights'),
  )
  night.dayAlpha = 0.0
  night.nightAlpha = 1.0
  night.brightness = 1.15
  scene.imageryLayers.add(day)
  scene.imageryLayers.add(night)
  installImageryFallback(viewer, [day, night])

  // Lighting and atmosphere.
  globe.baseColor = Color.fromCssColorString(LOOK.baseColor)
  globe.enableLighting = true
  globe.dynamicAtmosphereLighting = true
  globe.dynamicAtmosphereLightingFromSun = true
  globe.showGroundAtmosphere = true
  globe.atmosphereLightIntensity = LOOK.atmosphereLightIntensity
  globe.lightingFadeOutDistance = LOOK.lightingFadeOutDistance
  globe.lightingFadeInDistance = LOOK.lightingFadeInDistance
  globe.depthTestAgainstTerrain = false

  if (scene.skyAtmosphere) {
    scene.skyAtmosphere.show = true
    scene.skyAtmosphere.saturationShift = 0.08
    scene.skyAtmosphere.brightnessShift = -0.02
  }
  scene.backgroundColor = Color.BLACK
  if (scene.sun) scene.sun.show = true
  if (scene.moon) scene.moon.show = true
  scene.fog.enabled = false
  scene.highDynamicRange = false

  // Post-processing: anti-aliasing plus a restrained bloom (enabled per tier).
  scene.postProcessStages.fxaa.enabled = true
  const bloom = scene.postProcessStages.bloom
  bloom.enabled = false
  bloom.uniforms.glowOnly = false
  bloom.uniforms.contrast = LOOK.bloom.contrast
  bloom.uniforms.brightness = LOOK.bloom.brightness
  bloom.uniforms.delta = LOOK.bloom.delta
  bloom.uniforms.sigma = LOOK.bloom.sigma
  bloom.uniforms.stepSize = LOOK.bloom.stepSize

  // Camera feel.
  const ctrl = scene.screenSpaceCameraController
  ctrl.minimumZoomDistance = LOOK.minimumZoomDistance
  ctrl.maximumZoomDistance = LOOK.maximumZoomDistance
  ctrl.enableCollisionDetection = true
  ctrl.inertiaSpin = 0.93
  ctrl.inertiaZoom = 0.85

  applyQuality(viewer, 'medium')

  return viewer
}

/**
 * Render-cost ladder. 'medium' is the safe default (CSS-pixel resolution,
 * light MSAA, no bloom); main.ts steps up or down from measured frame times.
 * The jump from 'medium' to 'high' is roughly 4× the fragment work on a
 * Retina display, plus two full-screen blur passes for bloom.
 */
export type Quality = 'low' | 'medium' | 'high'
export const QUALITY_ORDER: Quality[] = ['low', 'medium', 'high']

export function applyQuality(viewer: Viewer, q: Quality): void {
  const { scene } = viewer
  const dpr = window.devicePixelRatio || 1
  switch (q) {
    case 'high':
      viewer.useBrowserRecommendedResolution = false
      viewer.resolutionScale = Math.min(dpr, 2)
      scene.msaaSamples = 4
      scene.postProcessStages.bloom.enabled = true
      break
    case 'medium':
      viewer.useBrowserRecommendedResolution = true
      viewer.resolutionScale = 1
      scene.msaaSamples = 2
      scene.postProcessStages.bloom.enabled = false
      break
    case 'low':
      viewer.useBrowserRecommendedResolution = true
      viewer.resolutionScale = 0.8
      scene.msaaSamples = 1
      scene.postProcessStages.bloom.enabled = false
      break
  }
}

/**
 * Opening move: place the camera so the terminator crosses the visible disc,
 * then ease in from deep space.
 */
export function playIntro(viewer: Viewer, now: Date): void {
  const hours = now.getUTCHours() + now.getUTCMinutes() / 60
  const subsolarLon = ((12 - hours) * 15 + 540) % 360 - 180
  const lon = ((subsolarLon + 50 + 540) % 360) - 180
  const lat = 22

  viewer.camera.setView({
    destination: Cartesian3.fromDegrees(lon, lat, LOOK.introStartHeight),
  })
  viewer.camera.flyTo({
    destination: Cartesian3.fromDegrees(lon, lat, LOOK.introEndHeight),
    duration: LOOK.introSeconds,
    easingFunction: EasingFunction.QUADRATIC_OUT,
  })
}

/**
 * Swap to the bundled low-resolution imagery when the streamed tiles fail
 * repeatedly (offline, blocked network). Returns nothing; purely a safeguard.
 */
function installImageryFallback(viewer: Viewer, layers: ImageryLayer[]): void {
  let failures = 0
  let swapped = false
  const swap = async () => {
    if (swapped) return
    swapped = true
    for (const l of layers) viewer.scene.imageryLayers.remove(l, true)
    const provider = await TileMapServiceImageryProvider.fromUrl(
      buildModuleUrl('Assets/Textures/NaturalEarthII'),
    )
    viewer.scene.imageryLayers.add(new ImageryLayer(provider))
    console.warn('[orbint] GIBS imagery unavailable, using bundled Natural Earth II')
  }
  for (const l of layers) {
    l.imageryProvider.errorEvent.addEventListener(() => {
      failures += 1
      if (failures >= 6) void swap()
    })
  }
}
