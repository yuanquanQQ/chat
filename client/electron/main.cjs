const { app, BrowserWindow, Menu } = require('electron')
const path = require('node:path')

function createWindow() {
  const win = new BrowserWindow({ width: 1200, height: 800, minWidth: 900, minHeight: 620, autoHideMenuBar: true, webPreferences: { preload: path.join(__dirname, 'preload.cjs'), contextIsolation: true, sandbox: true } })
  if (process.env.VITE_DEV_SERVER_URL) win.loadURL(process.env.VITE_DEV_SERVER_URL)
  else win.loadFile(path.join(__dirname, '../dist/index.html'))
}
app.whenReady().then(() => {
  Menu.setApplicationMenu(null)
  createWindow()
})
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit() })

