@echo off
chcp 65001 >nul
rem 注册为 Windows 服务并启动，等同于 slate.exe -d；需要管理员权限，请用「以管理员身份运行」打开命令提示符。
cd /d "%~dp0"
slate.exe -d %*
