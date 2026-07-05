# Ariadne native screenshot / pinned image 第 3 轮修改记录

- 修改角色：第 3 轮小修改 subagent
- 模型配置要求：gpt-5.5 xhigh, service_tier=priority
- 修改时间：2026-07-04
- 仓库：`P:\workspace\glwlg\app\Ariadne`
- 上一轮主会话验证报告：`.codex-audit/native-capture-pinned-image-iteration-02-modification-2026-07-04/capturesmoke-full-maincheck.json`
- 本轮范围：`cmd/capturesmoke/main_windows.go`、`cmd/capturesmoke/main_windows_test.go`
- Git 操作：未暂存、未提交、未回滚他人改动

## 本轮处理的问题

第 2 轮后主会话复跑完整 smoke 时，overlay、pin、尺寸、内容、位置都通过，`pin_selection=345ms`，但 `drag_pinned_window` 一次失败：

- 失败 detail：`delta=0,0`
- 同一 smoke 前后多次成功，说明产品拖动能力本身存在，失败更像是验证器在贴图窗口刚出现时立刻拖动，命中 Wails draggable 尚未完成初始化的短窗口期。

## 修改文件

- `cmd/capturesmoke/main_windows.go`
  - 新增贴图窗口拖动验证 helper：`dragPinnedWindow()`。
  - 贴图窗口出现后先等待 `300ms`，更接近真实用户看到窗口后再拖动的节奏。
  - 单次拖动未产生足够位移时，重读窗口位置并只重试一次。
  - 仍使用位移阈值判定拖动是否成功：最终 `deltaX` 或 `deltaY` 至少一个达到 `20px` 才通过。
  - `drag_pinned_window` step detail 改为包含尝试次数、最终 delta 和每次尝试 delta，例如：
    - `attempts=2 delta=71,43 attempt_deltas=1:0,0;2:71,43`

- `cmd/capturesmoke/main_windows_test.go`
  - 新增 `TestDragPinnedWindowRetriesAfterZeroDelta`：
    - 模拟第一次拖动 `delta=0,0`。
    - 第二次重读窗口后移动成功。
    - 断言 detail 写明 `attempts=2`、最终 delta 和每次尝试 delta。
  - 新增 `TestDragPinnedWindowStillFailsAfterRetryWithoutMovement`：
    - 模拟两次拖动都没有位移。
    - 断言不会无条件通过，仍保留失败判定边界。

## 修改前后行为

- 修改前：
  - `drag_pinned_window` 在贴图窗口刚出现后立即拖动一次。
  - 只记录 `delta=x,y`。
  - 如果第一次拖动打到 draggable 初始化前的窗口期，会直接失败。

- 修改后：
  - `drag_pinned_window` 最多尝试 2 次。
  - 第一次拖动前短等待；第一次无有效位移时，重读窗口并进行第二次拖动。
  - 验证没有被跳过，也没有放松成永远通过；两次后最终位移仍不足 `20px` 会失败。
  - detail 能看出真实尝试过程和最终位移。

## 验证命令和结果

- `go test -count=1 ./cmd/capturesmoke`
  - 结果：通过。
  - 输出摘要：`ok ariadne/cmd/capturesmoke 0.560s`。

- `go run ./cmd/capturesmoke -exe bin/ariadne.exe -output .codex-audit/native-capture-pinned-image-iteration-03-modification-2026-07-04/capturesmoke-full.json -max-pin-ms 1200 -timeout-ms 15000`
  - 结果：通过，报告已写入 `capturesmoke-full.json`。
  - 关键结果：
    - `pass=true`
    - `open_overlay_alt_a`：通过，452ms，窗口 `Ariadne - 原生截图覆盖层`
    - `pin_selection`：通过，312ms，低于 1200ms
    - `check_capture_dimensions`：通过，`260x180`
    - `compare_capture_content`：通过，`match=100.00% mean_abs_diff=0.000 mode=strict`
    - `check_pin_position`：通过，`delta=15,15`
    - `drag_pinned_window`：通过，`attempts=2 delta=71,43 attempt_deltas=1:0,0;2:71,43`

## 是否重建 exe

- 未运行 `wails3 task windows:build`。
- 原因：本轮只修改 `cmd/capturesmoke` smoke 验证工具和它的测试，没有修改 Ariadne 产品代码或 native capture host；`go run ./cmd/capturesmoke` 会即时编译本轮 smoke 代码，`bin/ariadne.exe` 不需要重建。

## 未处理事项和残余风险

- 如果桌面环境极慢，Wails draggable 初始化超过本轮的短等待加一次重试窗口，`drag_pinned_window` 仍会失败；这是验证器保留的真实失败信号，而不是跳过。
- 该 smoke 仍依赖真实 Windows 桌面、全局热键和鼠标事件，远程桌面焦点或系统输入拦截仍可能影响验证结果。
- 当前工作区存在大量本轮范围外的既有未提交改动，本轮没有暂存、提交或回滚。
