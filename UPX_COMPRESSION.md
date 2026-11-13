# AirSP Windows 可执行文件压缩说明

本文记录 AirSP 版本编译产物的 UPX 压缩流程，供后续 agent 或维护者复用。

## 环境要求

- 在 AirSP 源码根目录执行命令。
- 安装 UPX，并确保 `upx` 已加入 `PATH`。
- 建议使用与本次验证相同或更新的版本（本次为 UPX 5.0.2）。

检查工具：

```powershell
upx --version
```

## 推荐流程

1. 先编译生成未压缩的 Windows amd64 产物，例如：

   ```text
   sealdice-core-ra-sc-gif.exe
   ```

2. 如果直接压缩失败（常见提示为 PE 段布局不兼容），先使用 `strip` 生成精简版，再压缩精简版。不要覆盖未压缩原件。

3. 在源码根目录执行：

   ```powershell
   upx --best --lzma sealdice-core-v161-AirSP-ra-sc-gif-stripped.exe
   ```

4. 压缩完成后必须进行完整性测试：

   ```powershell
   upx -t sealdice-core-v161-AirSP-ra-sc-gif-stripped.exe
   upx -l sealdice-core-v161-AirSP-ra-sc-gif-stripped.exe
   ```

   只有看到 `Tested 1 file.` 和 `[OK]` 才可交付。

5. 将通过测试的压缩文件复制为发布文件名；保留未压缩版用于回滚：

   ```powershell
   Copy-Item `
     .\sealdice-core-v161-AirSP-ra-sc-gif-stripped.exe `
     .\sealdice-core-v161-AirSP-ra-sc-gif.exe -Force
   ```

## 本次产物记录（2026-09-01）

| 文件 | 用途 | 大小 |
|---|---|---:|
| `sealdice-core-v161-AirSP-ra-sc-gif.exe` | 正式 UPX 压缩版 | 21,271,040 bytes |
| `sealdice-core-v161-AirSP-ra-sc-gif-stripped.exe` | 精简并 UPX 压缩版（源文件） | 21,271,040 bytes |
| `sealdice-core-v161-AirSP-ra-sc-gif-uncompressed.exe` | 未压缩回滚版 | 136,841,025 bytes |

本次压缩参数为 `--best --lzma`，由 77,555,200 bytes 压缩至 21,271,040 bytes（约 27.43%）。`upx -t` 已通过。

## 回滚与注意事项

- 若运行时出现启动或杀毒软件兼容性问题，使用 `sealdice-core-v161-AirSP-ra-sc-gif-uncompressed.exe`。
- 不要删除未压缩回滚版，除非已在其他位置完成备份。
- 不要对同一个文件重复执行压缩；已压缩文件可用 `upx -d <文件>` 解压。
- 压缩后仍应进行一次实际启动检查；`upx -t` 只能证明压缩数据可解包，不能替代运行测试。
