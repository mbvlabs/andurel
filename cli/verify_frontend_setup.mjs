import { createRequire } from 'node:module'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

// Configs and plugins may write to stdout; reserve it for the protocol record.
const writeResult = process.stdout.write.bind(process.stdout)
process.stdout.write = () => true
const result = { violations: [] }
const root = path.resolve(process.argv[2] || process.cwd())
let file = 'vite.config'
const violation = (message, fix) => result.violations.push({ rule: 'frontend-setup', file, message, fix })

function missingDependency(error) {
  const seen = new Set()
  while (error && !seen.has(error)) {
    seen.add(error)
    if (error.code === 'ERR_MODULE_NOT_FOUND' || error.code === 'MODULE_NOT_FOUND' ||
        (typeof error.message === 'string' && /cannot find package|failed to resolve (?:entry for )?package/i.test(error.message))) return true
    error = error.cause
  }
  return false
}

function originAllowed(value, origin) {
  if (value === true || value === '*') return true
  if (typeof value === 'string') return value === origin
  if (value instanceof RegExp) {
    value.lastIndex = 0
    return value.test(origin)
  }
  if (Array.isArray(value)) return value.some(item => originAllowed(item, origin))
  return false
}

async function checkReload(config, vite) {
  const reloadFile = path.join(root, 'tmp', 'bin', 'vite-reload-check')
  if (config.server.watch === null) {
    violation('The Vite server watcher is disabled, so runner reloads cannot work.', 'Enable server.watch.')
    return
  }
  const ignored = config.server.watch?.ignored
  if (ignored !== undefined) {
    const patterns = Array.isArray(ignored) ? ignored : [ignored]
    if (patterns.some(pattern => typeof pattern !== 'string' && !(pattern instanceof RegExp)) || typeof vite.createFilter !== 'function') {
      result.blocked ||= 'Cannot determine whether server.watch.ignored excludes the runner reload file.'
    } else {
      const included = vite.createFilter(null, patterns, { resolve: false })
      const paths = [reloadFile, path.dirname(reloadFile), path.dirname(path.dirname(reloadFile))]
      if (paths.some(item => !included(vite.normalizePath(item)))) {
        violation('The Vite watcher ignores the runner reload file or its parent directory.', 'Remove the runner tmp/bin path from server.watch.ignored.')
      }
    }
  }
  const plugin = config.plugins.find(item => item.name === 'andurel-reload')
  if (!plugin) {
    violation('The development config does not include the andurel-reload plugin.', 'Restore the runner reload plugin.')
    return
  }
  const hook = plugin.configureServer
  const handler = typeof hook === 'function' ? hook : hook?.handler
  if (typeof handler !== 'function') {
    result.blocked = 'Cannot exercise the andurel-reload configureServer hook.'
    return
  }
  const previous = process.env.ANDUREL_VITE_RELOAD_FILE
  const watched = []
  const listeners = []
  const messages = []
  const watcher = {
    add(value) { watched.push(value); return this },
    on(event, callback) { if (event === 'change') listeners.push(callback); return this },
  }
  try {
    process.env.ANDUREL_VITE_RELOAD_FILE = reloadFile
    await handler.call(plugin, { watcher, ws: { send: (...args) => messages.push(args) } })
    if (!listeners.length) {
      result.blocked = 'Cannot exercise the andurel-reload change listener.'
      return
    }
    for (const listener of listeners) await listener(`${reloadFile}.unrelated`)
    const unrelatedCount = messages.length
    for (const listener of listeners) await listener(reloadFile)
    const reload = messages.slice(unrelatedCount).some(args => args[0]?.type === 'full-reload' || args[0] === 'full-reload')
    if (!watched.flat().includes(reloadFile) || unrelatedCount || !reload) {
      violation('The andurel-reload plugin does not watch and reload only the runner reload file.', 'Restore the runner reload watcher and full-reload message.')
    }
  } catch {
    result.blocked = 'Cannot safely exercise the andurel-reload hook with a simulated server.'
  } finally {
    if (previous === undefined) delete process.env.ANDUREL_VITE_RELOAD_FILE
    else process.env.ANDUREL_VITE_RELOAD_FILE = previous
  }
}

async function main() {
  process.chdir(root)
  try {
    const pkg = JSON.parse(await readFile(path.join(root, 'package.json'), 'utf8'))
    const scripts = pkg.scripts || {}
    const validDev = typeof scripts.dev === 'string' && /^\s*vite\s*$/.test(scripts.dev)
    const validBuild = typeof scripts.build === 'string' && /^\s*vite\s+build(?:\s*&&\s*vite\s+build\s+--ssr\s+resources\/js\/ssr\.tsx?\s+--outDir\s+assets\/dist\/ssr)?\s*$/.test(scripts.build)
    const validClient = scripts['build:client'] === undefined || (typeof scripts['build:client'] === 'string' && /^\s*vite\s+build\s*$/.test(scripts['build:client']))
    if (!validDev || !validBuild || !validClient) {
      result.blocked = 'Cannot verify package.json Vite scripts with missing scripts, CLI overrides, or unsupported commands.'
    }
  } catch {
    result.blocked = 'Cannot inspect package.json Vite scripts.'
  }
  let vitePath
  try {
    vitePath = createRequire(path.join(root, 'package.json')).resolve('vite')
  } catch {
    result.blocked = 'Vite is not installed in the project.'
    result.missingDependencies = true
    return
  }
  let vite
  try {
    vite = await import(pathToFileURL(vitePath).href)
  } catch {
    result.blocked = 'The installed Vite package or its dependencies could not be loaded; reinstall the project dependencies.'
    result.missingDependencies = true
    return
  }
  // Older Vite versions do not support the runner loader and may write a
  // bundled config into node_modules while resolving it.
  if (Number.parseInt(vite.version, 10) < 7 || !Number.isFinite(Number.parseInt(vite.version, 10))) {
    result.blocked = 'Vite 7 or newer is required to inspect the config without bundling it to disk.'
    return
  }
  let dev, build
  const previousNodeEnv = process.env.NODE_ENV
  try {
    process.env.NODE_ENV = 'development'
    dev = await vite.resolveConfig({ root, logLevel: 'silent', configLoader: 'runner' }, 'serve', 'development', 'development')
    process.env.NODE_ENV = 'production'
    build = await vite.resolveConfig({ root, logLevel: 'silent', configLoader: 'runner' }, 'build', 'production', 'production')
  } catch (error) {
    if (missingDependency(error)) {
      result.blocked = 'Vite configuration dependencies could not be loaded.'
      result.missingDependencies = true
    } else {
      violation('Vite could not resolve the project configuration.', 'Fix the Vite configuration and its installed dependencies.')
    }
    return
  } finally {
    if (previousNodeEnv === undefined) delete process.env.NODE_ENV
    else process.env.NODE_ENV = previousNodeEnv
  }
  if (dev.configFile) file = path.relative(root, dev.configFile)
  const env = { ...vite.loadEnv('development', dev.envDir || root, ['INERTIA_VITE_DEV_URL', 'HOST_', 'PROTOCOL']), ...process.env }
  let devURL, primaryOrigin
  try {
    devURL = new URL(env.INERTIA_VITE_DEV_URL || 'http://localhost:5173/assets/dist')
    primaryOrigin = new URL(`${env.PROTOCOL || 'http'}://${env.HOST_PRIMARY || 'localhost:8080'}`).origin
  } catch {
    violation('The configured development URL or primary host is invalid.', 'Set valid INERTIA_VITE_DEV_URL, HOST_PRIMARY and PROTOCOL values.')
    return
  }
  if (dev.base !== '/assets/dist/') violation('The development Vite base is not /assets/dist/.', 'Set the serve base to /assets/dist/.')
  if (devURL.pathname.replace(/\/+$/, '') !== dev.base.replace(/\/+$/, '')) violation('The development URL path does not match the Vite base.', 'Set INERTIA_VITE_DEV_URL to the development origin followed by /assets/dist.')
  if (dev.server.origin !== devURL.origin) violation('The Vite server origin does not match INERTIA_VITE_DEV_URL.', 'Set server.origin to the development URL origin.')
  const port = Number(devURL.port || (devURL.protocol === 'https:' ? 443 : 80))
  if (Number(dev.server.port ?? 5173) !== port) violation('The Vite server port does not match INERTIA_VITE_DEV_URL.', 'Set server.port to the development URL port.')
  if (dev.server.strictPort !== true) violation('The Vite server does not require its configured port.', 'Set server.strictPort to true to prevent port drift.')
  const cors = dev.server.cors
  const origins = new Set([primaryOrigin])
  try {
    for (const [key, host] of Object.entries(env)) {
      if (key.startsWith('HOST_') && host) origins.add(new URL(`${env.PROTOCOL || 'http'}://${host}`).origin)
    }
  } catch {
    violation('A configured HOST_ value is invalid.', 'Set valid HOST_ values for the configured protocol.')
  }
  for (const origin of origins) {
    const allowed = cors === true || (cors && typeof cors === 'object' && (cors.origin === undefined || originAllowed(cors.origin, origin)))
    if (!allowed) violation(`The Vite server CORS configuration does not allow ${origin}.`, 'Allow this configured application origin in server.cors.')
  }
  if (build.build.ssr) violation('The production Vite build is configured for SSR instead of client assets.', 'Disable build.ssr for the client build.')
  if (build.build.write === false) violation('The production Vite build is configured not to write assets.', 'Enable build.write for the client build.')
  const outDir = path.resolve(build.root, build.build.outDir)
  if (outDir !== path.join(root, 'assets/dist')) violation('The production output directory is not assets/dist.', 'Set build.outDir to the project assets/dist directory.')
  if (typeof build.build.manifest !== 'string' || path.resolve(outDir, build.build.manifest) !== path.join(root, 'assets/dist/vite/manifest.json')) {
    violation('The production manifest is not assets/dist/vite/manifest.json.', 'Set build.manifest to vite/manifest.json.')
  }
  if (build.base !== './') violation('The production Vite base is not relative.', "Set the build base to './'.")
  const outputs = build.build.rollupOptions?.output
  for (const output of outputs ? Array.isArray(outputs) ? outputs : [outputs] : []) {
    if (output.file !== undefined) violation('A production Rollup output uses output.file instead of the required output directory.', 'Remove output.file and emit into assets/dist.')
    if (output.dir !== undefined && (typeof output.dir !== 'string' || path.resolve(root, output.dir) !== path.join(root, 'assets/dist'))) {
      violation('A production Rollup output directory is not assets/dist.', 'Set each output.dir to the project assets/dist directory.')
    }
    for (const key of ['entryFileNames', 'chunkFileNames', 'assetFileNames']) {
      const pattern = output[key]
      if (typeof pattern === 'function') {
        result.blocked ||= `Cannot prove content-hashed production filenames from dynamic ${key}.`
      } else if (typeof pattern === 'string' && !/\[hash(?::\d+)?\]/.test(pattern)) {
        violation(`Production ${key} does not include a content hash.`, `Keep Vite's default ${key} or include [hash] in its pattern.`)
      }
    }
  }
  const input = build.build.rollupOptions?.input
  const entries = typeof input === 'string' ? [input] : Array.isArray(input) ? input : input && typeof input === 'object' ? Object.values(input) : ['index.html']
  result.entryPoints = entries.filter(item => typeof item === 'string').map(item => path.resolve(build.root, item))
  await checkReload(dev, vite)
}

try {
  await main()
} catch {
  result.blocked ||= 'The frontend setup checker could not complete.'
}
writeResult(`ANDUREL_FRONTEND_SETUP=${JSON.stringify(result)}\n`)
