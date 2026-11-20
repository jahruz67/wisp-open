# Regenerate icon artwork only; does not build or launch the app.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$projectRoot = Split-Path -Parent $PSScriptRoot

function New-RoundedPath([single]$X, [single]$Y, [single]$Width, [single]$Height, [single]$Radius) {
    $path = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $diameter = $Radius * 2
    $path.AddArc($X, $Y, $diameter, $diameter, 180, 90)
    $path.AddArc(($X + $Width - $diameter), $Y, $diameter, $diameter, 270, 90)
    $path.AddArc(($X + $Width - $diameter), ($Y + $Height - $diameter), $diameter, $diameter, 0, 90)
    $path.AddArc($X, ($Y + $Height - $diameter), $diameter, $diameter, 90, 90)
    $path.CloseFigure()
    return $path
}

function New-IconBitmap([int]$Size, [string]$Color, [switch]$Tile) {
    # Draw from vector geometry at four times the target resolution.
    $canvas = [System.Drawing.Bitmap]::new(($Size * 4), ($Size * 4))
    $graphics = [System.Drawing.Graphics]::FromImage($canvas)
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.ScaleTransform(($Size * 4 / 128), ($Size * 4 / 128))
    if ($Tile) {
        $tilePath = New-RoundedPath 5 5 118 118 27
        $tileFill = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
            [System.Drawing.Point]::new(0, 5), [System.Drawing.Point]::new(0, 123),
            [System.Drawing.ColorTranslator]::FromHtml('#347cd5'),
            [System.Drawing.ColorTranslator]::FromHtml('#123b7a'))
        $rim = [System.Drawing.Pen]::new([System.Drawing.ColorTranslator]::FromHtml('#81b7ef'), 1.5)
        $graphics.FillPath($tileFill, $tilePath)
        $graphics.DrawPath($rim, $tilePath)
        $rim.Dispose()
        $tileFill.Dispose()
        $tilePath.Dispose()
        $graphics.TranslateTransform(20, 17)
        $graphics.ScaleTransform(0.69, 0.69)
    }
    $ink = [System.Drawing.ColorTranslator]::FromHtml($Color)
    $fill = [System.Drawing.SolidBrush]::new($ink)
    $stroke = [System.Drawing.Pen]::new($ink, 10)
    $stroke.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
    $stroke.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
    $capsule = New-RoundedPath 48 16 32 64 16
    $graphics.FillPath($fill, $capsule)
    $stand = [System.Drawing.Drawing2D.GraphicsPath]::new()
    $stand.AddLine(28, 56, 28, 64)
    $stand.AddArc(28, 28, 72, 72, 180, -180)
    $stand.AddLine(100, 64, 100, 56)
    $graphics.DrawPath($stroke, $stand)
    $graphics.DrawLine($stroke, 64, 100, 64, 116)
    $graphics.DrawLine($stroke, 46, 116, 82, 116)
    $stand.Dispose()
    $capsule.Dispose()
    $stroke.Dispose()
    $fill.Dispose()
    $graphics.Dispose()
    $result = [System.Drawing.Bitmap]::new($Size, $Size)
    $output = [System.Drawing.Graphics]::FromImage($result)
    $output.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $output.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $output.DrawImage($canvas, 0, 0, $Size, $Size)
    $output.Dispose()
    $canvas.Dispose()
    return $result
}

function Save-IconPNG([string]$Path, [int]$Size, [string]$Color, [switch]$Tile) {
    $bitmap = New-IconBitmap $Size $Color -Tile:$Tile
    try { $bitmap.Save((Join-Path $projectRoot $Path), [System.Drawing.Imaging.ImageFormat]::Png) }
    finally { $bitmap.Dispose() }
}

function Save-IconICO([string]$Path, [switch]$Tile) {
    $sizes = @(16, 20, 24, 32, 48, 64, 128, 256)
    $frames = foreach ($size in $sizes) {
        $bitmap = New-IconBitmap $size '#ffffff' -Tile:$Tile
        $buffer = [System.IO.MemoryStream]::new()
        try {
            $bitmap.Save($buffer, [System.Drawing.Imaging.ImageFormat]::Png)
            ,$buffer.ToArray()
        } finally { $buffer.Dispose(); $bitmap.Dispose() }
    }
    $stream = [System.IO.File]::Create((Join-Path $projectRoot $Path))
    $writer = [System.IO.BinaryWriter]::new($stream)
    try {
        $writer.Write([uint16]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]$sizes.Count)
        $offset = 6 + 16 * $sizes.Count
        for ($index = 0; $index -lt $sizes.Count; $index++) {
            $dimension = $sizes[$index] % 256
            $writer.Write([byte]$dimension)
            $writer.Write([byte]$dimension)
            $writer.Write([uint16]0)
            $writer.Write([uint16]1)
            $writer.Write([uint16]32)
            $writer.Write([uint32]$frames[$index].Length)
            $writer.Write([uint32]$offset)
            $offset += $frames[$index].Length
        }
        foreach ($frame in $frames) { $writer.Write([byte[]]$frame) }
    } finally { $writer.Dispose() }
}

Save-IconPNG 'build/appicon.png' 1024 '#ffffff' -Tile
Save-IconPNG 'frontend/src/assets/images/logo-universal.png' 512 '#ffffff' -Tile
Save-IconPNG 'internal/ui/tray/icon.png' 128 '#ffffff'
Save-IconPNG 'internal/ui/tray/icon_recording.png' 128 '#ff3b30'
Save-IconPNG 'internal/ui/tray/icon_transcribing.png' 128 '#ffcc00'
Save-IconICO 'build/windows/icon.ico' -Tile
Save-IconICO 'internal/ui/tray/icon.ico' -Tile
