# 文档检查脚本

## check_readme_metrics.py

对比各插件 `plugin/metrics.go` 中注册的监控项与插件 `README.md` 的“监控项”章节，检测 README 是否遗漏监控项。

### 用法

```bash
# 在 zabbix_plugin 根目录执行，检查全部插件
python3 scripts/check_readme_metrics.py

# 只检查指定插件
python3 scripts/check_readme_metrics.py address ports

# 指定插件根目录
python3 scripts/check_readme_metrics.py --root /path/to/zabbix_plugin
```

### 输出示例

```text
[OK]    address: README 已覆盖全部监控项
[OK]    conntrack: README 已覆盖全部监控项
[OK]    hardware: README 已覆盖全部监控项
[OK]    login: README 已覆盖全部监控项
[OK]    ports: README 已覆盖全部监控项
[OK]    tcpconnect: README 已覆盖全部监控项

检查通过：6 个插件的 README 均覆盖全部监控项。
```

存在遗漏时：

```text
[FAIL]  conntrack: 缺少 1 个监控项
        - conntrack.info

检查失败：请补充 README.md 的“监控项”章节。
```

### 退出码

| 退出码 | 说明 |
|---|---|
| `0` | 所有监控项均已在 README 中出现 |
| `1` | 存在遗漏的监控项，或 README / metrics.go 不存在 |

### 检查逻辑

1. 从 `plugin/metrics.go` 的 `metric.MetricSet{...}` 中提取监控项键值；
2. 将常量名（如 `keyConntrackInfo`）解析为实际键值（如 `conntrack.info`）；
3. 检查 README 中是否存在以反引号包裹的键值，例如 `` `conntrack.info` ``；
4. 任一插件存在遗漏时以退出码 `1` 结束，可直接用于 CI 或提交前检查。

### 适用插件

`address`、`conntrack`、`hardware`、`login`、`ports`、`tcpconnect`。

## check_markdown_fences.py

检查 Markdown 文件中的代码围栏（```` ``` ```` / `~~~`）是否成对闭合，避免编辑文档后遗漏结束围栏。

### 用法

```bash
# 检查根目录下全部 Markdown 文件
python3 scripts/check_markdown_fences.py

# 只检查指定文件
python3 scripts/check_markdown_fences.py address/README.md ports/README.md

# 指定搜索根目录
python3 scripts/check_markdown_fences.py --root /path/to/zabbix_plugin
```

### 输出示例

```text
[OK]    address/README.md
[OK]    address/zabbix Address 插件编写.md
...
检查通过：13 个 Markdown 文件的代码围栏均成对闭合。
```

存在未闭合围栏时：

```text
[FAIL]  conntrack/README.md: 未闭合的代码围栏（开始于第 71 行，共 1 个开启围栏）

检查失败：请补充或删除未闭合的代码围栏。
```

### 退出码

| 退出码 | 说明 |
|---|---|
| `0` | 所有 Markdown 文件的代码围栏均成对闭合 |
| `1` | 存在未闭合的代码围栏，或文件读取失败 |

### 检查逻辑

1. 按 CommonMark 规则识别最多 3 个前导空格的 ```` ``` ```` 或 `~~~` 围栏；
2. 开启围栏后，只有相同字符、长度不小于开启围栏且后面没有内容的行才视为闭合；
3. 文件结束时仍有未闭合围栏则报错，并输出开启行号；
4. 支持 `~~~` 外层包裹 ```` ``` ```` 内层的嵌套写法，不会误判。

## check_all.sh

统一执行上述两个检查脚本，适合本地提交前检查或 CI 使用。

### 用法

```bash
cd /xiaonuo/workspace/code/zabbix_plugin
./scripts/check_all.sh

# 查看帮助
./scripts/check_all.sh --help
```

### 输出示例

```text
===== README 监控项检查 =====
...
[PASS] README 监控项检查

===== Markdown 代码围栏检查 =====
...
[PASS] Markdown 代码围栏检查

全部检查通过。
```

### 退出码

| 退出码 | 说明 |
|---|---|
| `0` | 两个检查均通过 |
| `1` | 任一检查失败，或未找到 `python3` |
| `2` | 传入了未知参数 |

脚本会依次执行两个检查，即使前一个失败也会继续执行后一个，最后汇总退出码，方便一次看到全部问题。

### 新增文档检查

在 `scripts/` 下新增检查脚本后，只需在 `check_all.sh` 的“检查项”区域追加一行 `run_check`：

```bash
run_check "插件配置检查" \
    python3 "$SCRIPT_DIR/check_plugin_configs.py" --root "$ROOT_DIR"
```

`run_check` 会自动打印检查标题、执行脚本，并在失败时参与最终退出码汇总。
