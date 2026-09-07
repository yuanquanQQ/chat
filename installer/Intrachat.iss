#define AppName "企业内网聊天"
#define AppVersion "0.1.0"
#define AppPublisher "公司内部"
#define AppExeName "Intrachat.exe"

[Setup]
AppId={{9F9C82F7-2D90-4E98-8D9C-7FEF2A96D1B6}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={autopf}\Intrachat
DefaultGroupName={#AppName}
OutputDir=..\release
OutputBaseFilename=Intrachat-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest
UninstallDisplayIcon={app}\{#AppExeName}
WizardStyle=modern

[Files]
Source: "..\release-stage\Intrachat-win32-x64\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"

[Run]
Filename: "{app}\{#AppExeName}"; Description: "启动{#AppName}"; Flags: nowait postinstall skipifsilent
