param(
    [string]$Binary = ".\gh-dep-risk.exe",
    [string]$OutputGif = "docs\assets\demo.gif",
    [string]$OutputCast = "docs\assets\demo.cast",
    [int]$Width = 1040,
    [int]$Height = 720
)

$ErrorActionPreference = "Stop"

function Quote-ProcessArgument {
    param([string]$Value)

    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + ($Value -replace '\\', '\\' -replace '"', '\"') + '"'
}

function Invoke-DemoCommand {
    param(
        [string]$Executable,
        [string[]]$Arguments,
        [int[]]$ExpectedExitCodes
    )

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Executable
    $psi.Arguments = (($Arguments | ForEach-Object { Quote-ProcessArgument $_ }) -join " ")
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true

    $process = [System.Diagnostics.Process]::Start($psi)
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()

    if ($ExpectedExitCodes -notcontains $process.ExitCode) {
        throw "command '$Executable $($psi.Arguments)' exited $($process.ExitCode), expected $($ExpectedExitCodes -join ', ')"
    }

    $lines = @()
    if ($stdout.TrimEnd().Length -gt 0) {
        $lines += ($stdout -split "`r?`n")
    }
    if ($stderr.TrimEnd().Length -gt 0) {
        $lines += ($stderr -split "`r?`n")
    }
    if ($process.ExitCode -ne 0) {
        $lines += "[exit $($process.ExitCode), expected for unsupported bun.lockb]"
    }
    return $lines
}

function Add-CastEvent {
    param(
        [System.Collections.Generic.List[string]]$Events,
        [double]$Time,
        [string]$Text
    )

    $payload = @($Time, "o", $Text) | ConvertTo-Json -Compress
    $Events.Add($payload)
}

function Add-FrameState {
    param(
        [System.Collections.Generic.List[object]]$States,
        [System.Collections.Generic.List[object]]$Lines,
        [int]$Repeat = 1
    )

    for ($i = 0; $i -lt $Repeat; $i++) {
        $States.Add(@($Lines.ToArray()))
    }
}

$commands = @(
    @{
        Display = "$Binary pr 2 --repo rad1092/gh-dep-risk-smoke-matrix --path yarn-app --lang en --format human --no-registry"
        Args = @("pr", "2", "--repo", "rad1092/gh-dep-risk-smoke-matrix", "--path", "yarn-app", "--lang", "en", "--format", "human", "--no-registry")
        Expected = @(0)
    },
    @{
        Display = "$Binary pr 9 --repo rad1092/gh-dep-risk-smoke-matrix --lang en --format human --no-registry"
        Args = @("pr", "9", "--repo", "rad1092/gh-dep-risk-smoke-matrix", "--lang", "en", "--format", "human", "--no-registry")
        Expected = @(0)
    },
    @{
        Display = "$Binary pr 10 --repo rad1092/gh-dep-risk-smoke-matrix --lang en --format human --no-registry"
        Args = @("pr", "10", "--repo", "rad1092/gh-dep-risk-smoke-matrix", "--lang", "en", "--format", "human", "--no-registry")
        Expected = @(0)
    },
    @{
        Display = "$Binary pr 11 --repo rad1092/gh-dep-risk-smoke-matrix --lang en --format human --no-registry"
        Args = @("pr", "11", "--repo", "rad1092/gh-dep-risk-smoke-matrix", "--lang", "en", "--format", "human", "--no-registry")
        Expected = @(2)
    }
)

$outputGifPath = [System.IO.Path]::GetFullPath($OutputGif)
$outputCastPath = [System.IO.Path]::GetFullPath($OutputCast)
$outputGifDir = [System.IO.Path]::GetDirectoryName($outputGifPath)
$outputCastDir = [System.IO.Path]::GetDirectoryName($outputCastPath)
New-Item -ItemType Directory -Force -Path $outputGifDir, $outputCastDir | Out-Null

$castEvents = New-Object 'System.Collections.Generic.List[string]'
$frameStates = New-Object 'System.Collections.Generic.List[object]'
$visibleLines = New-Object 'System.Collections.Generic.List[object]'
$time = 0.0

foreach ($command in $commands) {
    $prompt = "$ " + $command.Display
    $visibleLines.Add([pscustomobject]@{ Text = $prompt; Kind = "prompt" })
    Add-CastEvent $castEvents $time ($prompt + "`r`n")
    Add-FrameState $frameStates $visibleLines 4
    $time += 0.8

    $outputLines = Invoke-DemoCommand -Executable $Binary -Arguments $command.Args -ExpectedExitCodes $command.Expected
    foreach ($line in $outputLines) {
        $kind = "output"
        if ($line -match 'unsupported|no supported dependency change|\[exit') {
            $kind = "warning"
        }
        $visibleLines.Add([pscustomobject]@{ Text = $line; Kind = $kind })
        Add-CastEvent $castEvents $time ($line + "`r`n")
        Add-FrameState $frameStates $visibleLines 1
        $time += 0.16
    }
    $visibleLines.Add([pscustomobject]@{ Text = ""; Kind = "blank" })
    Add-CastEvent $castEvents $time "`r`n"
    Add-FrameState $frameStates $visibleLines 5
    $time += 1.1
}
Add-FrameState $frameStates $visibleLines 10

$castHeader = @{
    version = 2
    width = 104
    height = 30
    timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    env = @{
        SHELL = "powershell"
        TERM = "xterm-256color"
    }
} | ConvertTo-Json -Compress

Set-Content -Path $outputCastPath -Value (($castHeader + "`n" + ($castEvents -join "`n")) + "`n") -Encoding utf8 -NoNewline

Add-Type -AssemblyName System.Drawing

$framesDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gh-dep-risk-demo-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $framesDir | Out-Null

$background = [System.Drawing.Color]::FromArgb(14, 20, 25)
$panel = [System.Drawing.Color]::FromArgb(4, 8, 12)
$border = [System.Drawing.Color]::FromArgb(55, 70, 78)
$titleColor = [System.Drawing.Color]::FromArgb(232, 238, 242)
$mutedColor = [System.Drawing.Color]::FromArgb(140, 153, 164)
$promptColor = [System.Drawing.Color]::FromArgb(112, 219, 169)
$outputColor = [System.Drawing.Color]::FromArgb(226, 232, 236)
$warningColor = [System.Drawing.Color]::FromArgb(255, 181, 118)

$titleFont = New-Object System.Drawing.Font("Segoe UI", 18, [System.Drawing.FontStyle]::Bold)
$metaFont = New-Object System.Drawing.Font("Segoe UI", 10, [System.Drawing.FontStyle]::Regular)
$monoFont = New-Object System.Drawing.Font("Consolas", 14, [System.Drawing.FontStyle]::Regular)

$lineHeight = 22
$terminalX = 36
$terminalY = 78
$terminalWidth = $Width - 72
$terminalHeight = $Height - 110
$maxLines = [Math]::Floor(($terminalHeight - 28) / $lineHeight)

try {
    for ($index = 0; $index -lt $frameStates.Count; $index++) {
        $bitmap = New-Object System.Drawing.Bitmap($Width, $Height)
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::ClearTypeGridFit
        $graphics.Clear($background)

        $titleBrush = New-Object System.Drawing.SolidBrush($titleColor)
        $mutedBrush = New-Object System.Drawing.SolidBrush($mutedColor)
        $panelBrush = New-Object System.Drawing.SolidBrush($panel)
        $borderPen = New-Object System.Drawing.Pen($border, 1)

        $graphics.DrawString("gh-dep-risk", $titleFont, $titleBrush, 36, 24)
        $graphics.DrawString("live dependency PR checks across Yarn, Bun, and unsupported fallback behavior", $metaFont, $mutedBrush, 176, 33)
        $graphics.FillRectangle($panelBrush, $terminalX, $terminalY, $terminalWidth, $terminalHeight)
        $graphics.DrawRectangle($borderPen, $terminalX, $terminalY, $terminalWidth, $terminalHeight)

        $lines = @($frameStates[$index])
        $start = [Math]::Max(0, $lines.Count - $maxLines)
        $y = $terminalY + 18
        for ($lineIndex = $start; $lineIndex -lt $lines.Count; $lineIndex++) {
            $line = $lines[$lineIndex]
            $color = $outputColor
            if ($line.Kind -eq "prompt") {
                $color = $promptColor
            } elseif ($line.Kind -eq "warning") {
                $color = $warningColor
            } elseif ($line.Kind -eq "blank") {
                $y += $lineHeight
                continue
            }
            $brush = New-Object System.Drawing.SolidBrush($color)
            $text = $line.Text
            if ($text.Length -gt 92) {
                $text = $text.Substring(0, 89) + "..."
            }
            $graphics.DrawString($text, $monoFont, $brush, $terminalX + 18, $y)
            $brush.Dispose()
            $y += $lineHeight
        }

        $titleBrush.Dispose()
        $mutedBrush.Dispose()
        $panelBrush.Dispose()
        $borderPen.Dispose()

        $framePath = Join-Path $framesDir ("frame-{0:D4}.png" -f $index)
        $bitmap.Save($framePath, [System.Drawing.Imaging.ImageFormat]::Png)
        $graphics.Dispose()
        $bitmap.Dispose()
    }

    $palettePath = Join-Path $framesDir "palette.png"
    $framePattern = Join-Path $framesDir "frame-%04d.png"
    & ffmpeg -y -hide_banner -loglevel error -framerate 3 -i $framePattern -vf "palettegen=stats_mode=diff" $palettePath
    if ($LASTEXITCODE -ne 0) {
        throw "ffmpeg palette generation failed"
    }
    & ffmpeg -y -hide_banner -loglevel error -framerate 3 -i $framePattern -i $palettePath -lavfi "paletteuse=dither=bayer:bayer_scale=5" -loop 0 $outputGifPath
    if ($LASTEXITCODE -ne 0) {
        throw "ffmpeg gif generation failed"
    }
}
finally {
    $titleFont.Dispose()
    $metaFont.Dispose()
    $monoFont.Dispose()
    Remove-Item -LiteralPath $framesDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Wrote $outputCastPath"
Write-Host "Wrote $outputGifPath"
