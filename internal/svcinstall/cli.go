package svcinstall

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// RunFromMain 是 main 包在发现 service 子命令时的入口，处理完直接 os.Exit。
func RunFromMain(args []string) {
	env, err := DefaultEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化环境失败: %v\n", err)
		os.Exit(1)
	}
	code := dispatch(os.Stdout, os.Stderr, env, args)
	os.Exit(code)
}

// RunCommand 供测试调用，可注入 Env 和参数，返回标准输出文案与错误（错误文案来自 stderr）。
func RunCommand(env *Env, args []string) (string, error) {
	var out, errOut strings.Builder
	code := dispatch(&out, &errOut, env, args)
	if code != 0 {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = fmt.Sprintf("命令失败，退出码 %d", code)
		}
		return out.String(), fmt.Errorf("%s", msg)
	}
	return out.String(), nil
}

// dispatch 根据子命令分派到 print、install、uninstall。
func dispatch(stdout, stderr io.Writer, env *Env, args []string) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "print":
		return cmdPrint(stdout, stderr, env, rest)
	case "install":
		return cmdInstall(stdout, stderr, env, rest)
	case "uninstall":
		return cmdUninstall(stdout, stderr, env, rest)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "未知子命令 %q\n\n", cmd)
		printUsage(stderr)
		return 1
	}
}

// cmdPrint 把服务文件内容打印到标准输出，不写盘。
func cmdPrint(stdout, stderr io.Writer, env *Env, args []string) int {
	opts, err := parseCommonFlags(args, true)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		printUsage(stderr)
		return 1
	}
	platform := opts.platform
	if platform == "" {
		platform, err = DetectPlatform(env.GOOS)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}
	content := env.ContentForPlatform(platform, opts.conf)
	fmt.Fprint(stdout, content)
	return 0
}

// cmdInstall 写入用户级服务文件并提示后续手工命令。
func cmdInstall(stdout, stderr io.Writer, env *Env, args []string) int {
	opts, err := parseCommonFlags(args, false)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		printUsage(stderr)
		return 1
	}
	platform, err := DetectPlatform(env.GOOS)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if opts.dryRun {
		content := env.ContentForPlatform(platform, opts.conf)
		path := ServiceFilePath(env.HomeDir, platform)
		fmt.Fprint(stdout, FormatDryRunInstall(env, platform, path, content))
		return 0
	}
	path, _, wr, err := InstallService(env, platform, opts.conf, false)
	if err != nil {
		fmt.Fprintf(stderr, "安装失败: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, FormatInstallFollowUp(env, platform, path, wr))
	return 0
}

// cmdUninstall 删除服务文件并提示手工停止命令。
func cmdUninstall(stdout, stderr io.Writer, env *Env, args []string) int {
	opts, err := parseUninstallFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		printUsage(stderr)
		return 1
	}
	platform, err := DetectPlatform(env.GOOS)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	path := ServiceFilePath(env.HomeDir, platform)
	hadFile := fileExists(path)
	if opts.dryRun {
		fmt.Fprint(stdout, FormatDryRunUninstall(env, platform, path, hadFile))
		return 0
	}
	path, hadFile, err = UninstallService(env, platform, false)
	if err != nil {
		fmt.Fprintf(stderr, "卸载失败: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, FormatUninstallFollowUp(env, platform, path, hadFile))
	return 0
}

// fileExists 判断路径是否存在，避免把其它错误当成不存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// flagOpts 保存 print/install 解析后的旗标。
type flagOpts struct {
	conf     string
	platform string
	dryRun   bool
}

// uninstallOpts 保存 uninstall 解析后的旗标。
type uninstallOpts struct {
	dryRun bool
}

// parseCommonFlags 解析 -conf、--platform、--dry-run；allowPlatform 为 true 时接受 --platform。
func parseCommonFlags(args []string, allowPlatform bool) (flagOpts, error) {
	opts := flagOpts{} // conf 为空表示用默认目录 ~/.slate-kanban，在 ServiceInput 里换成绝对路径
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "-conf":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("-conf 需要跟一个目录路径")
			}
			if err := validateConfArg(args[i+1]); err != nil {
				return opts, err
			}
			opts.conf = args[i+1]
			i += 2
		case "--platform":
			if !allowPlatform {
				return opts, fmt.Errorf("子命令 install/uninstall 不支持 --platform")
			}
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--platform 需要跟 launchd 或 systemd")
			}
			p, err := ParsePlatformFlag(args[i+1])
			if err != nil {
				return opts, err
			}
			opts.platform = p
			i += 2
		case "--dry-run":
			opts.dryRun = true
			i++
		case "-h", "--help":
			return opts, fmt.Errorf("用法见 slate service help")
		default:
			return opts, fmt.Errorf("无法识别的参数 %q", a)
		}
	}
	return opts, nil
}

// validateConfArg 校验 -conf 的取值：不能为空、不能过长、不能含换行等控制字符，
// 否则会把服务文件写坏，甚至多出一行不该有的配置。
func validateConfArg(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("-conf 不能是空字符串")
	}
	if len(v) > maxConfArgLen {
		return fmt.Errorf("-conf 路径过长，最多 %d 个字节", maxConfArgLen)
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("-conf 路径里不能有换行或其他控制字符")
		}
	}
	return nil
}

// parseUninstallFlags 解析 uninstall 仅有的 --dry-run。
func parseUninstallFlags(args []string) (uninstallOpts, error) {
	opts := uninstallOpts{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			opts.dryRun = true
		case "-h", "--help":
			return opts, fmt.Errorf("用法见 slate service help")
		default:
			return opts, fmt.Errorf("无法识别的参数 %q", args[i])
		}
	}
	return opts, nil
}

// printUsage 输出简短用法到 w。
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  slate service print [--platform launchd|systemd] [-conf 目录]")
	fmt.Fprintln(w, "  slate service install [-conf 目录] [--dry-run]")
	fmt.Fprintln(w, "  slate service uninstall [--dry-run]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "安装只会写入当前用户级服务文件；加载、启用、停止须在本机手工执行 launchctl 或 systemctl。")
}
