!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef SOURCE
  !error "SOURCE must point to the Windows bundle directory"
!endif
!ifndef OUTFILE
  !define OUTFILE "ytm-playlist-dl-setup.exe"
!endif

!define APPNAME "ytm-playlist-dl"
!define COMPANYNAME "Mikan256"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\ytm-playlist-dl"

Name "${APPNAME} ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\ytm-playlist-dl"
InstallDirRegKey HKCU "Software\ytm-playlist-dl" "InstallDir"
RequestExecutionLevel user
Unicode true
SetCompressor /SOLID lzma

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Japanese"
!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"
  File "${SOURCE}/ytm-playlist-dl-desktop.exe"
  File "${SOURCE}/LICENSE"
  File "${SOURCE}/THIRD_PARTY_NOTICES.md"
  File "${SOURCE}/README.md"

  SetOutPath "$INSTDIR\bin"
  File "${SOURCE}/bin/yt-dlp.exe"
  File "${SOURCE}/bin/ffmpeg.exe"
  File "${SOURCE}/bin/ffprobe.exe"

  SetOutPath "$INSTDIR\LICENSES"
  File /r "${SOURCE}/LICENSES/*.*"

  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateShortcut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\ytm-playlist-dl-desktop.exe"
  CreateDirectory "$SMPROGRAMS\${APPNAME}"
  CreateShortcut "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk" "$INSTDIR\ytm-playlist-dl-desktop.exe"
  CreateShortcut "$SMPROGRAMS\${APPNAME}\アンインストール.lnk" "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "Software\ytm-playlist-dl" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayName" "${APPNAME}"
  WriteRegStr HKCU "${UNINSTKEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTKEY}" "Publisher" "${COMPANYNAME}"
  WriteRegStr HKCU "${UNINSTKEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  Delete "$DESKTOP\${APPNAME}.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\アンインストール.lnk"
  RMDir "$SMPROGRAMS\${APPNAME}"

  DeleteRegKey HKCU "${UNINSTKEY}"
  DeleteRegKey HKCU "Software\ytm-playlist-dl"

  RMDir /r "$INSTDIR"
SectionEnd
