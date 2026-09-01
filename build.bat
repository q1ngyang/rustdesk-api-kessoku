@echo off
setlocal
for /f %%i in ('node --version') do set KESSOKU_NODE_VERSION=%%i
for /f %%i in ('npm --version') do set KESSOKU_NPM_VERSION=%%i
if not "%KESSOKU_NODE_VERSION%"=="v24.15.0" (
  echo Node.js 24.15.0 is required 1>&2
  exit /b 1
)
if not "%KESSOKU_NPM_VERSION%"=="11.12.1" (
  echo npm 11.12.1 is required 1>&2
  exit /b 1
)
set "KESSOKU_FRONTEND_EVIDENCE=%TEMP%\kessoku-frontends-%RANDOM%-%RANDOM%"
mkdir "%KESSOKU_FRONTEND_EVIDENCE%"
if errorlevel 1 exit /b 1
call :build_frontend admin-web admin-web resources\admin admin-web\LICENSE
if errorlevel 1 exit /b 1
call :build_frontend web-client web-client resources\client web-client\LICENSE complete
if errorlevel 1 exit /b 1
rmdir /s /q release
if not exist release\NUL mkdir release
set GO111MODULE=on
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/api --instanceName api --exclude http/controller/admin
if errorlevel 1 exit /b 1
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/apimain.go --output docs/admin --instanceName admin --exclude http/controller/api
if errorlevel 1 exit /b 1
for /f %%i in ('git rev-parse HEAD') do set KESSOKU_SOURCE_COMMIT=%%i
if not defined KESSOKU_SOURCE_COMMIT exit /b 1
for /f %%i in ('git show -s --format^=%%cI HEAD') do set KESSOKU_BUILD_TIME=%%i
if not defined KESSOKU_BUILD_TIME exit /b 1
for /f %%i in (resources\version) do set KESSOKU_RELEASE_TAG=%%i
set "KESSOKU_RELEASE_VERSION=%KESSOKU_RELEASE_TAG:~1%"
go build -trimpath -buildvcs=true -ldflags "-X github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo.Version=%KESSOKU_RELEASE_VERSION% -X github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo.GitCommit=%KESSOKU_SOURCE_COMMIT% -X github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo.BuildTime=%KESSOKU_BUILD_TIME%" -o release/kessoku-api.exe ./cmd
if errorlevel 1 exit /b 1
release\kessoku-api.exe version --json > release\VERSION.json
if errorlevel 1 exit /b 1
findstr /L /C:"\"version\":\"%KESSOKU_RELEASE_VERSION%\"" release\VERSION.json >NUL
if errorlevel 1 exit /b 1
findstr /L /C:"\"git_commit\":\"%KESSOKU_SOURCE_COMMIT%\"" release\VERSION.json >NUL
if errorlevel 1 exit /b 1
go version -m release\kessoku-api.exe > release\GO-BUILD-INFO.txt
if errorlevel 1 exit /b 1
findstr /L /C:"github.com/q1ngyang/rustdesk-api-kessoku/v3/cmd" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
findstr /L /C:"vcs.revision=%KESSOKU_SOURCE_COMMIT%" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
findstr /L /C:"vcs.modified=false" release\GO-BUILD-INFO.txt >NUL
if errorlevel 1 exit /b 1
xcopy resources\i18n release\resources\i18n /E /I /Y
xcopy resources\public release\resources\public /E /I /Y
xcopy resources\templates release\resources\templates /E /I /Y
copy resources\version release\resources\version
xcopy resources\admin release\resources\admin /E /I /Y
if errorlevel 1 exit /b 1
xcopy resources\client release\resources\client /E /I /Y
if errorlevel 1 exit /b 1
if not exist "release\resources\client\third-party-licenses\@bufbuild-protobuf-2.9.0.txt" exit /b 1
if exist release\resources\web\NUL exit /b 1
if exist release\resources\web2\NUL exit /b 1
xcopy docs release\docs /E /I /Y
xcopy conf release\conf /E /I /Y
copy README.md release\README.md
copy README_EN.md release\README_EN.md
copy README.zh-CN.md release\README.zh-CN.md
rem Guides and release records were copied above into release\docs\.
copy RELEASE_STATUS release\RELEASE_STATUS
copy LICENSE release\LICENSE
copy admin-web\LICENSE release\ADMIN-WEB-LICENSE
copy web-client\LICENSE release\WEB-CLIENT-LICENSE
copy web-client\NOTICE.md release\WEB-CLIENT-NOTICE.md
xcopy "%KESSOKU_FRONTEND_EVIDENCE%" release\frontend-evidence /E /I /Y
mkdir release\runtime
mkdir release\data
rmdir /s /q "%KESSOKU_FRONTEND_EVIDENCE%"
endlocal
exit /b 0

:build_frontend
set "KESSOKU_FRONTEND_NAME=%~1"
set "KESSOKU_FRONTEND_SOURCE=%~2"
set "KESSOKU_FRONTEND_RUNTIME=%~3"
set "KESSOKU_FRONTEND_LICENSE=%~4"
set "KESSOKU_FRONTEND_SBOM_SCOPE=%~5"
if not exist "%KESSOKU_FRONTEND_SOURCE%\package-lock.json" exit /b 1
if not exist "%KESSOKU_FRONTEND_LICENSE%" exit /b 1
pushd "%KESSOKU_FRONTEND_SOURCE%"
call npm ci
if errorlevel 1 exit /b 1
call npm run lint
if errorlevel 1 exit /b 1
call npm test
if errorlevel 1 exit /b 1
call npm audit --omit=dev --audit-level=high
if errorlevel 1 exit /b 1
call npm audit signatures
if errorlevel 1 exit /b 1
call npm run build
if errorlevel 1 exit /b 1
powershell -NoProfile -Command "Get-ChildItem dist -Recurse -File | Sort-Object FullName | Get-FileHash -Algorithm SHA256 | Select-Object Hash,Path | ConvertTo-Json -Compress | Set-Content -Encoding ascii '%KESSOKU_FRONTEND_EVIDENCE%\%KESSOKU_FRONTEND_NAME%-dist-1.sha256'"
if errorlevel 1 exit /b 1
call npm run build
if errorlevel 1 exit /b 1
powershell -NoProfile -Command "Get-ChildItem dist -Recurse -File | Sort-Object FullName | Get-FileHash -Algorithm SHA256 | Select-Object Hash,Path | ConvertTo-Json -Compress | Set-Content -Encoding ascii '%KESSOKU_FRONTEND_EVIDENCE%\%KESSOKU_FRONTEND_NAME%-dist-2.sha256'"
if errorlevel 1 exit /b 1
fc /b "%KESSOKU_FRONTEND_EVIDENCE%\%KESSOKU_FRONTEND_NAME%-dist-1.sha256" "%KESSOKU_FRONTEND_EVIDENCE%\%KESSOKU_FRONTEND_NAME%-dist-2.sha256" >NUL
if errorlevel 1 exit /b 1
if "%KESSOKU_FRONTEND_SBOM_SCOPE%"=="complete" (
  call npm sbom --sbom-format cyclonedx > "%KESSOKU_FRONTEND_EVIDENCE%\kessoku-%KESSOKU_FRONTEND_NAME%.cdx.json"
) else (
  call npm sbom --omit=dev --sbom-format cyclonedx > "%KESSOKU_FRONTEND_EVIDENCE%\kessoku-%KESSOKU_FRONTEND_NAME%.cdx.json"
)
if errorlevel 1 exit /b 1
node -e "const fs=require('fs');const s=JSON.parse(fs.readFileSync(process.argv[1]));if(s.components.some(c=^>!(c.licenses^|^|[]).length))throw new Error('missing production licence metadata')" "%KESSOKU_FRONTEND_EVIDENCE%\kessoku-%KESSOKU_FRONTEND_NAME%.cdx.json"
if errorlevel 1 exit /b 1
if "%KESSOKU_FRONTEND_NAME%"=="web-client" node -e "const fs=require('fs');const s=JSON.parse(fs.readFileSync(process.argv[1]));if(!s.components.some(c=^>c.name==='@bufbuild/protobuf'^&^&c.version==='2.9.0'^&^&(c.licenses^|^|[]).length))throw new Error('web client runtime dependency missing from SBOM')" "%KESSOKU_FRONTEND_EVIDENCE%\kessoku-%KESSOKU_FRONTEND_NAME%.cdx.json"
if errorlevel 1 exit /b 1
popd
if exist "%KESSOKU_FRONTEND_RUNTIME%\NUL" rmdir /s /q "%KESSOKU_FRONTEND_RUNTIME%"
xcopy "%KESSOKU_FRONTEND_SOURCE%\dist" "%KESSOKU_FRONTEND_RUNTIME%" /E /I /Y
if errorlevel 1 exit /b 1
if "%KESSOKU_FRONTEND_NAME%"=="web-client" if not exist "%KESSOKU_FRONTEND_RUNTIME%\third-party-licenses\@bufbuild-protobuf-2.9.0.txt" exit /b 1
copy "%KESSOKU_FRONTEND_LICENSE%" "%KESSOKU_FRONTEND_EVIDENCE%\%KESSOKU_FRONTEND_NAME%-LICENSE"
exit /b 0
