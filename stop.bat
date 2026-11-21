@echo off
chcp 65001 >nul
rem 停止并删除 Windows 服务，等同于 slate.exe -stop；需要管理员权限。
cd /d "%~dp0"
slate.exe -stop
