#define AppName "企业内网聊天"
#define AppVersion "0.1.0"
#define AppPublisher "公司内部"
#define AppExeName "Intrachat.exe"

[Setup]
AppId={{9F9C82F7-2D90-4E98-8D9C-7FEF2A96D1B6}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
; 固定每用户安装目录（无需管理员权限，重复安装/升级稳定覆盖）
DefaultDirName={localappdata}\Programs\Intrachat
DisableDirPage=yes
DefaultGroupName={#AppName}
OutputDir=..\release
OutputBaseFilename=Intrachat-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest
UninstallDisplayIcon={app}\{#AppExeName}
WizardStyle=modern
; --- 升级安装：自动替换旧版本 ---
UsePreviousAppDir=yes
CloseApplications=yes
RestartApplications=no

[Files]
Source: "..\release-stage\Intrachat-win32-x64\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"

[Run]
Filename: "{app}\{#AppExeName}"; Description: "启动{#AppName}"; Flags: nowait postinstall skipifsilent

[Code]
const
  UninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{9F9C82F7-2D90-4E98-8D9C-7FEF2A96D1B6}_is1';

// 安装开始前：关闭正在运行的旧程序，并静默卸载已安装的旧版本，
// 保证新版本完整覆盖，不残留旧文件。
function InitializeSetup(): Boolean;
var
  uninstaller: String;
  ResultCode: Integer;
begin
  Result := True;
  Exec('taskkill', '/f /im Intrachat.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  if RegQueryStringValue(HKCU, UninstallKey, 'UninstallString', uninstaller) then
  begin
    Exec(RemoveQuotes(uninstaller), '/VERYSILENT /NORESTART', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end
  else if RegQueryStringValue(HKLM, UninstallKey, 'UninstallString', uninstaller) then
  begin
    Exec(RemoveQuotes(uninstaller), '/VERYSILENT /NORESTART', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
end;
