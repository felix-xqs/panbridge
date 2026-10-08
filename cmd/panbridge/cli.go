package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/felix-xqs/panbridge/internal/limits"
	"github.com/felix-xqs/panbridge/internal/metrics"
	"github.com/felix-xqs/panbridge/internal/model"
	"github.com/felix-xqs/panbridge/internal/preview"
	"github.com/felix-xqs/panbridge/internal/provider"
	"github.com/felix-xqs/panbridge/internal/scan"
)

const helpText = `云桥 PanBridge — 本地目录同步验证版

用法：
  panbridge scan --root <目录> [--json] [--scan-pause 5ms]
  panbridge dry-run --left <目录> --right <目录> [--json] [--limit 50]
  panbridge version

scan 统计目录元数据和资源占用。
dry-run 预览初次合并计划，同名差异需要确认。
两个命令只读取指定目录，输出计划与统计；不会执行文件同步。
`

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 || args[0] == "version" || args[0] == "--version" {
		_, err := fmt.Fprintf(out, "PanBridge %s\n", version)
		return err
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, helpText)
		return err
	}
	var err error
	switch args[0] {
	case "scan":
		err = scanCommand(ctx, args[1:], out, errOut)
	case "dry-run":
		err = previewCommand(ctx, args[1:], out, errOut)
	default:
		err = fmt.Errorf("未知命令 %q，使用 panbridge help 查看用法", args[0])
	}
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func scanOptions(delay time.Duration, cpuBudget float64) (scan.Options, error) {
	if delay < 0 {
		return scan.Options{}, fmt.Errorf("scan-pause 不能为负数")
	}
	options := scan.DefaultOptions()
	options.YieldDelay = delay
	budget, err := limits.NewBudget(cpuBudget)
	if err != nil {
		return scan.Options{}, err
	}
	options.Budget = budget
	return options, nil
}

func scanCommand(ctx context.Context, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(errOut)
	root := flags.String("root", "", "要扫描的本地目录")
	jsonOutput := flags.Bool("json", false, "输出 JSON 统计")
	pause := flags.Duration("scan-pause", 5*time.Millisecond, "每处理 512 项后的暂停时长，0 表示关闭固定暂停")
	cpuBudget := flags.Float64("cpu-budget", 25, "CPU 平均目标，单核百分比；0 表示关闭 CPU 节流")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" || flags.NArg() != 0 {
		return fmt.Errorf("用法：panbridge scan --root <目录>")
	}
	options, err := scanOptions(*pause, *cpuBudget)
	if err != nil {
		return err
	}
	source, err := provider.NewLocalProvider(*root)
	if err != nil {
		return err
	}
	recorder := metrics.Start()
	summary, err := scan.Tree(ctx, source, options, func(model.Entry) error { return nil })
	if err != nil {
		return err
	}
	measurement := recorder.Finish()
	if *jsonOutput {
		return json.NewEncoder(out).Encode(struct {
			Command string          `json:"command"`
			Scan    scan.Summary    `json:"scan"`
			Metrics metrics.Summary `json:"metrics"`
		}{"scan", summary, measurement})
	}
	if _, err := fmt.Fprintf(out, "扫描完成：%d 个文件，%d 个目录，%d 个链接或特殊项；文件大小合计 %d 字节。\n", summary.Files, summary.Directories, summary.Unsupported, summary.Bytes); err != nil {
		return err
	}
	return printMetrics(out, measurement)
}

func previewCommand(ctx context.Context, args []string, out, errOut io.Writer) (resultErr error) {
	flags := flag.NewFlagSet("dry-run", flag.ContinueOnError)
	flags.SetOutput(errOut)
	left := flags.String("left", "", "左侧本地目录")
	right := flags.String("right", "", "右侧本地目录")
	jsonOutput := flags.Bool("json", false, "输出逐行 JSON，最后一行是汇总")
	limit := flags.Int("limit", 50, "最多显示多少条计划，0 表示只显示汇总，最大 10000")
	pause := flags.Duration("scan-pause", 5*time.Millisecond, "每处理 512 项后的暂停时长")
	cpuBudget := flags.Float64("cpu-budget", 25, "CPU 平均目标，单核百分比；0 表示关闭 CPU 节流")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *left == "" || *right == "" || flags.NArg() != 0 {
		return fmt.Errorf("用法：panbridge dry-run --left <目录> --right <目录>")
	}
	if *limit < 0 || *limit > 10000 {
		return fmt.Errorf("limit 必须在 0 到 10000 之间")
	}
	options, err := scanOptions(*pause, *cpuBudget)
	if err != nil {
		return err
	}
	lp, err := provider.NewLocalProvider(*left)
	if err != nil {
		return err
	}
	rp, err := provider.NewLocalProvider(*right)
	if err != nil {
		return err
	}
	recorder := metrics.Start()
	index, err := preview.NewIndex(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, index.Close()) }()
	ls, err := index.Scan(ctx, preview.Left, lp, options)
	if err != nil {
		return err
	}
	rs, err := index.Scan(ctx, preview.Right, rp, options)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	shown := int64(0)
	plan, err := index.Plan(ctx, func(operation preview.Operation) error {
		if shown >= int64(*limit) {
			return nil
		}
		shown++
		if *jsonOutput {
			return encoder.Encode(struct {
				Type string `json:"type"`
				preview.Operation
			}{"operation", operation})
		}
		_, err := fmt.Fprintf(out, "%s  %q  (%s)\n", actionLabel(operation.Action), operation.Path, reasonLabel(operation.Reason))
		return err
	})
	if err != nil {
		return err
	}
	measurement := recorder.Finish()
	if *jsonOutput {
		return encoder.Encode(struct {
			Type    string              `json:"type"`
			Mode    string              `json:"mode"`
			Left    scan.Summary        `json:"left"`
			Right   scan.Summary        `json:"right"`
			Plan    preview.PlanSummary `json:"plan"`
			Shown   int64               `json:"shown"`
			Omitted int64               `json:"omitted"`
			Metrics metrics.Summary     `json:"metrics"`
		}{"summary", "initial_merge_metadata_only", ls, rs, plan, shown, plan.ProposedOperations() - shown, measurement})
	}
	if _, err := fmt.Fprintf(out, "初次合并预览：左→右 %d 个文件，右→左 %d 个文件，待建目录 %d，待确认 %d，跳过 %d。\n元数据匹配 %d 项（尚未验证内容）；显示 %d 条，省略 %d 条。\n", plan.CopiesToRight, plan.CopiesToLeft, plan.DirectoriesRight+plan.DirectoriesLeft, plan.NeedsReview, plan.Skipped, plan.MetadataMatches, shown, plan.ProposedOperations()-shown); err != nil {
		return err
	}
	return printMetrics(out, measurement)
}

func actionLabel(action preview.Action) string {
	return map[preview.Action]string{
		preview.CopyToRight: "复制到右侧", preview.CopyToLeft: "复制到左侧",
		preview.MkdirRight: "右侧建目录", preview.MkdirLeft: "左侧建目录",
		preview.Review: "待确认", preview.Skip: "跳过",
	}[action]
}

func reasonLabel(reason string) string {
	return map[string]string{
		"left_only": "仅左侧存在", "right_only": "仅右侧存在",
		"file_directory_conflict": "文件与目录同名",
		"unsupported_entry":       "链接或特殊文件", "blocked_ancestor": "父路径需要处理",
		"baseline_or_content_check_required": "需要同步历史或内容校验",
	}[reason]
}

func printMetrics(out io.Writer, measurement metrics.Summary) error {
	if _, err := fmt.Fprintf(out, "耗时 %.3f 秒；当前 Go 堆 %.1f MiB", measurement.ElapsedSeconds, float64(measurement.HeapBytes)/(1<<20)); err != nil {
		return err
	}
	if measurement.CPUPercent != nil {
		if _, err := fmt.Fprintf(out, "；平均 CPU %.1f%%（单核口径）；进程峰值内存 %.1f MiB", *measurement.CPUPercent, float64(measurement.PeakRSSBytes)/(1<<20)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out)
	return err
}
