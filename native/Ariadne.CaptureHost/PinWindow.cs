using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Threading;
using WpfBrushes = System.Windows.Media.Brushes;
using WpfImage = System.Windows.Controls.Image;

namespace Ariadne.CaptureHost;

internal sealed class PinWindow : Window
{
    private const double MinZoom = 0.25;
    private const double MaxZoom = 3.0;
    private const int ShadowPadding = 10;

    private readonly byte[] _pngBytes;
    private readonly BitmapSource _image;
    private readonly string _nativePinId;
    private readonly string _callbackPipeName;
    private readonly Border _imageFrame;
    private readonly WpfImage _imageView;
    private readonly Border _feedback;
    private readonly TextBlock _feedbackText;
    private readonly DispatcherTimer _feedbackTimer;
    private readonly int _baseWidth;
    private readonly int _baseHeight;
    private readonly double _scaleX;
    private readonly double _scaleY;
    private string _ocrText = "";
    private string _tempImagePath = "";
    private double _zoom = 1;
    private bool _shadowEnabled = true;

    public PinWindow(byte[] pngBytes, int x, int y, int width, int height, string nativePinId, string callbackPipeName)
    {
        _pngBytes = pngBytes;
        _nativePinId = nativePinId.Trim();
        _callbackPipeName = callbackPipeName.Trim();
        _baseWidth = Math.Max(1, width);
        _baseHeight = Math.Max(1, height);
        _image = LoadImage(pngBytes);

        Title = "截图贴图";
        WindowStyle = WindowStyle.None;
        ResizeMode = ResizeMode.NoResize;
        ShowInTaskbar = false;
        Topmost = true;
        AllowsTransparency = true;
        Background = WpfBrushes.Transparent;
        Cursor = Cursors.Arrow;
        Focusable = true;
        MinWidth = 1;
        MinHeight = 1;

        var scale = NativeMethods.DpiScaleForPoint(x, y);
        _scaleX = Math.Max(0.1, scale.ScaleX);
        _scaleY = Math.Max(0.1, scale.ScaleY);
        Width = Math.Max(1, (_baseWidth + ShadowPadding * 2) / _scaleX);
        Height = Math.Max(1, (_baseHeight + ShadowPadding * 2) / _scaleY);
        Left = (x - ShadowPadding) / _scaleX;
        Top = (y - ShadowPadding) / _scaleY;

        _imageView = new WpfImage
        {
            Source = _image,
            Stretch = Stretch.Fill,
            SnapsToDevicePixels = true,
            UseLayoutRounding = true
        };
        RenderOptions.SetBitmapScalingMode(_imageView, BitmapScalingMode.HighQuality);

        _imageFrame = new Border
        {
            Background = WpfBrushes.Transparent,
            Child = _imageView
        };
        ApplyShadowState();

        _feedbackText = new TextBlock
        {
            Foreground = WpfBrushes.White,
            FontSize = 12,
            TextWrapping = TextWrapping.NoWrap,
            Margin = new Thickness(10, 5, 10, 5)
        };
        _feedback = new Border
        {
            Background = new SolidColorBrush(Color.FromArgb(215, 18, 24, 38)),
            CornerRadius = new CornerRadius(6),
            HorizontalAlignment = HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Bottom,
            Margin = new Thickness(8),
            Child = _feedbackText,
            Visibility = Visibility.Collapsed,
            IsHitTestVisible = false
        };

        var root = new Grid();
        root.Children.Add(_imageFrame);
        root.Children.Add(_feedback);
        Content = root;

        _feedbackTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(1400) };
        _feedbackTimer.Tick += (_, _) =>
        {
            _feedbackTimer.Stop();
            _feedback.Visibility = Visibility.Collapsed;
        };

        SourceInitialized += (_, _) => PlaceForImageOrigin(x, y);
        Loaded += (_, _) => Focus();
        Closed += (_, _) => DeleteTempImage();
        MouseLeftButtonDown += HandleLeftButtonDown;
        MouseWheel += HandleMouseWheel;
        PreviewKeyDown += HandleKeyDown;
        ContextMenuOpening += (_, _) => ContextMenu = BuildContextMenu();
        ContextMenu = BuildContextMenu();
    }

    private ContextMenu BuildContextMenu()
    {
        var menu = new ContextMenu();
        menu.Items.Add(MenuItem("复制图片", (_, _) => CopyImage()));
        menu.Items.Add(MenuItem("OCR 文字识别", async (_, _) => await RecognizeOcrAsync(), _callbackPipeName.Length == 0));
        menu.Items.Add(MenuItem("复制 OCR 全文", (_, _) => CopyOcrText(), string.IsNullOrWhiteSpace(_ocrText)));
        menu.Items.Add(new Separator());
        menu.Items.Add(MenuItem("放大", (_, _) => ZoomBy(0.1), _zoom >= MaxZoom));
        menu.Items.Add(MenuItem("缩小", (_, _) => ZoomBy(-0.1), _zoom <= MinZoom));
        menu.Items.Add(MenuItem("原始比例", (_, _) => ResetZoom(), Math.Abs(_zoom - 1) < 0.001));
        menu.Items.Add(MenuItem(_shadowEnabled ? "关闭阴影" : "打开阴影", (_, _) => ToggleShadow()));
        menu.Items.Add(new Separator());
        menu.Items.Add(MenuItem("关闭贴图", (_, _) => Close()));
        return menu;
    }

    private static MenuItem MenuItem(string header, RoutedEventHandler handler, bool disabled = false)
    {
        var item = new MenuItem
        {
            Header = header,
            IsEnabled = !disabled
        };
        item.Click += handler;
        return item;
    }

    private static BitmapSource LoadImage(byte[] pngBytes)
    {
        using var stream = new MemoryStream(pngBytes);
        var image = new BitmapImage();
        image.BeginInit();
        image.CacheOption = BitmapCacheOption.OnLoad;
        image.StreamSource = stream;
        image.EndInit();
        image.Freeze();
        return image;
    }

    private void HandleLeftButtonDown(object sender, MouseButtonEventArgs eventArgs)
    {
        Focus();
        if (eventArgs.ClickCount >= 2)
        {
            Close();
            return;
        }
        try
        {
            DragMove();
        }
        catch
        {
            // DragMove can throw when Windows has already ended the button drag.
        }
    }

    private void HandleMouseWheel(object sender, MouseWheelEventArgs eventArgs)
    {
        eventArgs.Handled = true;
        ZoomBy(eventArgs.Delta > 0 ? 0.08 : -0.08);
    }

    private void HandleKeyDown(object sender, KeyEventArgs eventArgs)
    {
        if (eventArgs.Key == Key.Escape)
        {
            eventArgs.Handled = true;
            Close();
            return;
        }
        if (eventArgs.Key == Key.D0 && (Keyboard.Modifiers & ModifierKeys.Control) == ModifierKeys.Control)
        {
            eventArgs.Handled = true;
            ResetZoom();
        }
    }

    private void CopyImage()
    {
        try
        {
            Clipboard.SetImage(_image);
            ShowFeedback("图片已复制");
        }
        catch
        {
            ShowFeedback("复制失败");
        }
    }

    private async Task RecognizeOcrAsync()
    {
        try
        {
            ShowFeedback("OCR 中");
            var response = await PinActionClient.SendAsync(_callbackPipeName, new PinActionRequest
            {
                Action = "ocr_text",
                NativePinId = _nativePinId,
                ImagePath = EnsureTempImagePath()
            });
            if (!response.Ok)
            {
                ShowFeedback(string.IsNullOrWhiteSpace(response.Message) ? "OCR 识别失败" : response.Message);
                return;
            }
            _ocrText = response.Text.Trim();
            if (_ocrText.Length == 0)
            {
                ShowFeedback("未识别到文字");
                return;
            }
            Clipboard.SetText(_ocrText);
            ShowFeedback("OCR 文本已复制");
        }
        catch
        {
            ShowFeedback("OCR 识别失败");
        }
    }

    private void CopyOcrText()
    {
        if (string.IsNullOrWhiteSpace(_ocrText))
        {
            ShowFeedback("没有可复制的 OCR 文本");
            return;
        }
        try
        {
            Clipboard.SetText(_ocrText);
            ShowFeedback("OCR 文本已复制");
        }
        catch
        {
            ShowFeedback("复制失败");
        }
    }

    private string EnsureTempImagePath()
    {
        if (_tempImagePath.Length > 0 && File.Exists(_tempImagePath))
        {
            return _tempImagePath;
        }
        _tempImagePath = Path.Combine(Path.GetTempPath(), "ariadne-native-pin-" + _nativePinId + ".png");
        File.WriteAllBytes(_tempImagePath, _pngBytes);
        return _tempImagePath;
    }

    private void DeleteTempImage()
    {
        if (_tempImagePath.Length == 0)
        {
            return;
        }
        try
        {
            File.Delete(_tempImagePath);
        }
        catch
        {
            // Temporary OCR image cleanup is best-effort.
        }
    }

    private void ZoomBy(double delta)
    {
        SetZoom(Math.Clamp(_zoom + delta, MinZoom, MaxZoom));
    }

    private void ResetZoom()
    {
        SetZoom(1);
    }

    private void ToggleShadow()
    {
        var rect = NativeMethods.WindowRect(this);
        var imageX = rect.X + CurrentShadowPadding();
        var imageY = rect.Y + CurrentShadowPadding();
        _shadowEnabled = !_shadowEnabled;
        ApplyShadowState();
        PlaceForImageOrigin(imageX, imageY);
    }

    private void ApplyShadowState()
    {
        var padding = CurrentShadowPadding();
        _imageFrame.Margin = new Thickness(
            padding / _scaleX,
            padding / _scaleY,
            padding / _scaleX,
            padding / _scaleY);
        _imageFrame.Effect = _shadowEnabled ? NativeVisuals.Shadow(62, 18, 4) : null;
    }

    private void SetZoom(double zoom)
    {
        if (Math.Abs(zoom - _zoom) < 0.001)
        {
            return;
        }
        _zoom = zoom;
        var rect = NativeMethods.WindowRect(this);
        PlaceForImageOrigin(rect.X + CurrentShadowPadding(), rect.Y + CurrentShadowPadding());
    }

    private void PlaceForImageOrigin(int x, int y)
    {
        var padding = CurrentShadowPadding();
        NativeMethods.PlaceWindowInPhysicalPixels(
            this,
            x - padding,
            y - padding,
            Math.Max(1, (int)Math.Round(_baseWidth * _zoom)) + padding * 2,
            Math.Max(1, (int)Math.Round(_baseHeight * _zoom)) + padding * 2);
    }

    private int CurrentShadowPadding()
    {
        return _shadowEnabled ? ShadowPadding : 0;
    }

    private void ShowFeedback(string text)
    {
        _feedbackText.Text = text;
        _feedback.Visibility = Visibility.Visible;
        _feedbackTimer.Stop();
        _feedbackTimer.Start();
    }
}
