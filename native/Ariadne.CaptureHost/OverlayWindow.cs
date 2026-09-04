using Microsoft.Win32;
using System.Globalization;
using System.IO;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Effects;
using System.Windows.Media.Imaging;
using System.Windows.Shapes;
using System.Windows.Threading;
using WpfButton = System.Windows.Controls.Button;
using WpfBrushes = System.Windows.Media.Brushes;
using WpfColor = System.Windows.Media.Color;
using WpfCursors = System.Windows.Input.Cursors;
using WpfImage = System.Windows.Controls.Image;
using WpfKeyEventArgs = System.Windows.Input.KeyEventArgs;
using WpfMouseEventArgs = System.Windows.Input.MouseEventArgs;
using WpfOrientation = System.Windows.Controls.Orientation;
using WpfPoint = System.Windows.Point;
using WpfRectangle = System.Windows.Shapes.Rectangle;
using WpfSaveFileDialog = Microsoft.Win32.SaveFileDialog;
using WpfSize = System.Windows.Size;

namespace Ariadne.CaptureHost;

internal enum OverlayInteractionMode
{
    None,
    Creating,
    Moving,
    Resizing,
    DrawingAnnotation,
    MovingAnnotation
}

internal enum ResizeAnchor
{
    None,
    TopLeft,
    Top,
    TopRight,
    Right,
    BottomRight,
    Bottom,
    BottomLeft,
    Left
}

internal enum AnnotationTool
{
    None,
    Rect,
    Line,
    Arrow,
    Pen,
    Highlight,
    Mosaic,
    Text,
    Number,
    Eraser,
    Select
}

internal sealed class OverlayWindow : Window
{
    private const long MagnifierContentFrameMs = 16;

    private static readonly string[] ColorPalette =
    [
        "#dc2626",
        "#f97316",
        "#facc15",
        "#22c55e",
        "#1f2933",
        "#2563eb",
        "#7c3aed",
        "#111827",
        "#ffffff"
    ];

    private readonly ScreenCapture _capture;
    private readonly CaptureRequest _request;
    private readonly Canvas _overlay = new();
    private readonly Canvas _annotationLayer = new();
    private readonly WpfRectangle _topMask = Mask();
    private readonly WpfRectangle _leftMask = Mask();
    private readonly WpfRectangle _rightMask = Mask();
    private readonly WpfRectangle _bottomMask = Mask();
    private readonly Border _selection = new();
    private readonly DropShadowEffect _selectionIdleEffect = new()
    {
        Color = WpfColor.FromRgb(31, 41, 51),
        BlurRadius = 10,
        ShadowDepth = 0,
        Opacity = 0.75
    };
    private readonly Border _toolbar = new();
    private readonly Border _selectionSize = new();
    private readonly TextBlock _selectionSizeText = new();
    private readonly Border _hint = new();
    private readonly Border _feedback = new();
    private readonly TextBlock _feedbackText = new();
    private readonly Border _magnifier = new();
    private readonly WpfImage _magnifierImage = new();
    private readonly TextBlock _magnifierText = new();
    private readonly WrapPanel _primaryToolbarRow = new();
    private readonly WrapPanel _secondaryToolbarRow = new();
    private readonly DispatcherTimer _feedbackTimer = new();
    private readonly List<(Border Element, ResizeAnchor Anchor)> _handles = new();
    private readonly List<(WpfButton Button, AnnotationTool Tool)> _toolButtons = [];
    private readonly List<WpfButton> _selectionButtons = [];
    private readonly List<WpfButton> _operationButtons = [];
    private readonly List<WpfButton> _colorButtons = [];
    private readonly List<AnnotationOperation> _operations = [];
    private readonly List<AnnotationOperation> _redoOperations = [];
    private WpfButton? _closeButton;
    private WpfButton? _ocrButton;
    private Border? _textEditorHost;
    private TextBox? _textEditor;
    private WpfPoint _textEditorPoint;
    private WpfPoint _start;
    private WpfPoint _end;
    private WpfPoint _interactionStart;
    private WpfPoint _annotationStart;
    private WpfPoint _annotationMoveStart;
    private WpfPoint _lastPoint;
    private Rect _interactionOrigin;
    private OverlayInteractionMode _mode;
    private ResizeAnchor _resizeAnchor;
    private AnnotationTool _tool = AnnotationTool.None;
    private AnnotationOperation? _draftOperation;
    private AnnotationOperation? _annotationMoveOrigin;
    private Slider? _thicknessSlider;
    private TextBlock? _thicknessText;
    private int _selectedOperationIndex = -1;
    private int _thickness = 3;
    private int _numberCounter = 1;
    private string _annotationColor = "#dc2626";
    private string _lastSelectionSizeText = "";
    private int _lastSelectionSizeTextLength = -1;
    private WpfSize _lastSelectionSizeMeasure = WpfSize.Empty;
    private long _lastMagnifierContentMs;
    private int _lastMagnifierPhysicalX = int.MinValue;
    private int _lastMagnifierPhysicalY = int.MinValue;
    private bool _selectionDragVisualsActive;
    private bool _editMode;
    private bool _completed;
    private bool _ocrBusy;
    private bool _redactBusy;

    public OverlayWindow(ScreenCapture capture, CaptureRequest request)
    {
        _capture = capture;
        _request = request;

        Title = "Ariadne - 原生截图覆盖层";
        WindowStyle = WindowStyle.None;
        ResizeMode = ResizeMode.NoResize;
        Topmost = true;
        ShowInTaskbar = false;
        Left = capture.Bounds.X / capture.ScaleX;
        Top = capture.Bounds.Y / capture.ScaleY;
        Width = capture.DipWidth;
        Height = capture.DipHeight;
        Cursor = WpfCursors.Cross;
        Background = WpfBrushes.Black;

        _feedbackTimer.Interval = TimeSpan.FromMilliseconds(1400);
        _feedbackTimer.Tick += (_, _) =>
        {
            _feedbackTimer.Stop();
            _feedback.Visibility = Visibility.Collapsed;
        };

        Content = BuildContent();
        SourceInitialized += (_, _) => NativeMethods.PlaceWindowInPhysicalPixels(this, capture.Bounds.X, capture.Bounds.Y, capture.Bounds.Width, capture.Bounds.Height);
        Closed += (_, _) =>
        {
            if (!_completed)
            {
                Complete(new CaptureResponse { Ok = false, Canceled = true, Message = "已取消截图" });
            }
        };
    }

    public event Action<CaptureResponse>? Completed;

    public void CloseFromCoordinator()
    {
        _completed = true;
        Close();
    }

    private Grid BuildContent()
    {
        var root = new Grid { ClipToBounds = true };
        root.Children.Add(new WpfImage
        {
            Source = _capture.Source,
            Stretch = Stretch.Fill,
            SnapsToDevicePixels = true
        });

        _overlay.Background = WpfBrushes.Transparent;
        _overlay.Focusable = true;
        _overlay.Children.Add(_topMask);
        _overlay.Children.Add(_leftMask);
        _overlay.Children.Add(_rightMask);
        _overlay.Children.Add(_bottomMask);

        _selection.BorderBrush = NativeVisuals.Brush(31, 41, 51);
        _selection.BorderThickness = new Thickness(1.4);
        _selection.Background = NativeVisuals.Brush(22, 255, 255, 255);
        if (_selectionIdleEffect.CanFreeze)
        {
            _selectionIdleEffect.Freeze();
        }
        _selection.Effect = _selectionIdleEffect;
        _selection.Visibility = Visibility.Collapsed;
        _selection.IsHitTestVisible = false;
        _overlay.Children.Add(_selection);

        _annotationLayer.Visibility = Visibility.Collapsed;
        _annotationLayer.IsHitTestVisible = false;
        _annotationLayer.ClipToBounds = true;
        _overlay.Children.Add(_annotationLayer);

        _selectionSize.Background = NativeVisuals.Brush(238, 250, 250, 250);
        _selectionSize.BorderBrush = NativeVisuals.Brush(180, 255, 255, 255);
        _selectionSize.BorderThickness = new Thickness(1);
        _selectionSize.CornerRadius = new CornerRadius(10);
        _selectionSize.Padding = new Thickness(8, 3, 8, 3);
        _selectionSize.Effect = NativeVisuals.Shadow(38, 18, 5);
        _selectionSize.Visibility = Visibility.Collapsed;
        _selectionSize.IsHitTestVisible = false;
        _selectionSizeText.Foreground = NativeVisuals.Brush(31, 41, 51);
        _selectionSizeText.FontFamily = new FontFamily("Consolas, Cascadia Mono, Segoe UI");
        _selectionSizeText.FontSize = 11;
        _selectionSizeText.FontWeight = FontWeights.SemiBold;
        _selectionSize.Child = _selectionSizeText;
        _overlay.Children.Add(_selectionSize);

        foreach (var handle in BuildHandles())
        {
            _handles.Add(handle);
            _overlay.Children.Add(handle.Element);
        }

        _toolbar.Background = NativeVisuals.Brush(236, 250, 250, 250);
        _toolbar.BorderBrush = NativeVisuals.Brush(180, 255, 255, 255);
        _toolbar.BorderThickness = new Thickness(1);
        _toolbar.CornerRadius = new CornerRadius(8);
        _toolbar.Padding = new Thickness(6);
        _toolbar.Effect = NativeVisuals.Shadow(70, 34, 11);
        _toolbar.Visibility = Visibility.Collapsed;
        _toolbar.Cursor = Cursors.Arrow;
        _toolbar.Child = ToolbarContent();
        _overlay.Children.Add(_toolbar);

        _closeButton = NativeVisuals.IconButton("lucide:x", "取消截图", () => Complete(new CaptureResponse { Ok = false, Canceled = true, Message = "已取消截图" }), false, 32);
        _closeButton.Background = NativeVisuals.Brush(226, 250, 250, 250);
        _closeButton.Foreground = NativeVisuals.Brush(63, 63, 70);
        _closeButton.Effect = NativeVisuals.Shadow(42, 20, 6);
        _closeButton.Cursor = Cursors.Arrow;
        _overlay.Children.Add(_closeButton);

        BuildMagnifier();
        _overlay.Children.Add(_magnifier);

        _hint.Background = NativeVisuals.Brush(226, 250, 250, 250);
        _hint.BorderBrush = NativeVisuals.Brush(172, 255, 255, 255);
        _hint.BorderThickness = new Thickness(1);
        _hint.CornerRadius = new CornerRadius(16);
        _hint.Padding = new Thickness(10, 6, 10, 6);
        _hint.Effect = NativeVisuals.Shadow(52, 24, 7);
        _hint.IsHitTestVisible = false;
        _hint.Child = new TextBlock
        {
            Text = "拖拽选择区域  Enter 复制  Shift+Enter 打码  P 贴图  Q 扫码  R/A/L 标注  B/H/M 画笔  T/N/E 文字  C 取色并退出  Shift 切换 RGB/HEX",
            Foreground = NativeVisuals.Brush(63, 63, 70),
            FontSize = 12,
            TextWrapping = TextWrapping.Wrap
        };
        _overlay.Children.Add(_hint);

        _feedback.Background = NativeVisuals.Brush(238, 17, 24, 39);
        _feedback.BorderBrush = NativeVisuals.Brush(34, 255, 255, 255);
        _feedback.BorderThickness = new Thickness(1);
        _feedback.CornerRadius = new CornerRadius(16);
        _feedback.Padding = new Thickness(11, 6, 11, 6);
        _feedback.Effect = NativeVisuals.Shadow(78, 28, 7);
        _feedback.Visibility = Visibility.Collapsed;
        _feedback.IsHitTestVisible = false;
        _feedbackText.Foreground = NativeVisuals.Brush(244, 244, 245);
        _feedbackText.FontSize = 12;
        _feedbackText.FontWeight = FontWeights.SemiBold;
        _feedback.Child = _feedbackText;
        _overlay.Children.Add(_feedback);

        _overlay.MouseLeftButtonDown += BeginSelection;
        _overlay.MouseMove += MoveSelection;
        _overlay.MouseLeftButtonUp += EndInteraction;
        _overlay.PreviewMouseRightButtonDown += CancelCaptureWithRightClick;
        _overlay.PreviewMouseWheel += AdjustThicknessWithWheel;
        PreviewKeyDown += HandlePreviewKeyDown;
        _overlay.KeyDown += HandleKeyDown;
        _overlay.SizeChanged += (_, _) => UpdateSelectionVisuals();
        _overlay.Loaded += (_, _) =>
        {
            PositionOverlayChrome();
            UpdateToolStates();
            _overlay.Focus();
        };
        root.Children.Add(_overlay);
        return root;
    }

    private void HandlePreviewKeyDown(object sender, WpfKeyEventArgs eventArgs)
    {
        if (eventArgs.Key != Key.Escape)
        {
            return;
        }

        CancelCapture();
        eventArgs.Handled = true;
    }

    private void CancelCaptureWithRightClick(object sender, MouseButtonEventArgs eventArgs)
    {
        CancelCapture();
        eventArgs.Handled = true;
    }

    private void CancelCapture()
    {
        Complete(new CaptureResponse { Ok = false, Canceled = true, Message = "已取消截图" });
    }

    private UIElement ToolbarContent()
    {
        var panel = new StackPanel { Orientation = WpfOrientation.Vertical };
        var primary = _primaryToolbarRow;
        primary.Orientation = WpfOrientation.Horizontal;
        primary.Children.Add(ActionButton("lucide:qr-code", "扫码 (Q)", () => Finish("qr")));
        primary.Children.Add(Separator());
        primary.Children.Add(ToolButton("lucide:square", "矩形 (R)", AnnotationTool.Rect));
        primary.Children.Add(ToolButton("lucide:arrow-up-right", "箭头 (A)", AnnotationTool.Arrow));
        primary.Children.Add(ToolButton("lucide:minus", "直线 (L)", AnnotationTool.Line));
        primary.Children.Add(ToolButton("lucide:pencil", "画笔 (B)", AnnotationTool.Pen));
        primary.Children.Add(ToolButton("lucide:highlighter", "荧光笔 (H)", AnnotationTool.Highlight));
        primary.Children.Add(ToolButton("lucide:grid-3x3", "马赛克 (M)", AnnotationTool.Mosaic));
        primary.Children.Add(ToolButton("lucide:type", "文字 (T)", AnnotationTool.Text));
        primary.Children.Add(ToolButton("lucide:hash", "序号 (N)", AnnotationTool.Number));
        primary.Children.Add(ToolButton("lucide:eraser", "橡皮擦 (E)", AnnotationTool.Eraser));
        primary.Children.Add(ToolButton("lucide:mouse-pointer-2", "选择/移动标注 (V)", AnnotationTool.Select));
        primary.Children.Add(Separator());
        primary.Children.Add(ActionButton("lucide:save", "另存为 (Ctrl+S)", () => Finish("save_as")));
        primary.Children.Add(ActionButton("lucide:rotate-ccw", "撤销 (Ctrl+Z)", UndoAnnotation));
        primary.Children.Add(ActionButton("lucide:redo-2", "重做 (Ctrl+Y)", RedoAnnotation));
        primary.Children.Add(ActionButton("lucide:circle-slash-2", "清空标注", ClearAnnotations));
        primary.Children.Add(ActionButton("lucide:trash-2", "删除选中标注", DeleteSelectedAnnotation));
        primary.Children.Add(ActionButton("lucide:x", "重新选择", ResetSelection));
        primary.Children.Add(Separator());
        _ocrButton = ActionButton("lucide:file-text", "OCR 并复制文字", () => _ = RecognizeSelectionOcrAsync());
        primary.Children.Add(_ocrButton);
        primary.Children.Add(ActionButton("lucide:copy", "复制到剪贴板 (Enter)", () => Finish("copy"), true));
        primary.Children.Add(ActionButton("lucide:shield-check", "打码并复制 (Shift+Enter)", () => Finish("redact_copy")));
        primary.Children.Add(ActionButton("lucide:pin", "贴图 (P)", () => Finish("pin")));
        panel.Children.Add(primary);

        var secondary = _secondaryToolbarRow;
        secondary.Orientation = WpfOrientation.Horizontal;
        secondary.Margin = new Thickness(0, 6, 0, 0);
        foreach (var color in ColorPalette)
        {
            secondary.Children.Add(ColorSwatch(color));
        }
        secondary.Children.Add(new TextBlock
        {
            Text = "粗细",
            Foreground = NativeVisuals.Brush(82, 82, 91),
            FontSize = 12,
            Margin = new Thickness(9, 6, 4, 0)
        });
        _thicknessText = new TextBlock
        {
            Text = _thickness.ToString(CultureInfo.InvariantCulture),
            Foreground = NativeVisuals.Brush(31, 41, 51),
            FontSize = 12,
            FontWeight = FontWeights.SemiBold,
            Width = 22,
            Margin = new Thickness(4, 6, 0, 0)
        };
        _thicknessSlider = new Slider
        {
            Minimum = 1,
            Maximum = 24,
            Value = _thickness,
            Width = 108,
            TickFrequency = 1,
            IsSnapToTickEnabled = true,
            Margin = new Thickness(2, 0, 0, 0),
            Cursor = Cursors.Arrow,
            Focusable = false
        };
        _thicknessSlider.ValueChanged += (_, args) =>
        {
            SetThickness((int)Math.Round(args.NewValue), false);
        };
        secondary.Children.Add(_thicknessSlider);
        secondary.Children.Add(_thicknessText);
        panel.Children.Add(secondary);
        return panel;
    }

    private WpfButton ActionButton(string glyph, string tooltip, Action action, bool primary = false)
    {
        var button = NativeVisuals.IconButton(glyph, tooltip, action, primary, 30);
        button.Margin = new Thickness(2, 0, 2, 0);
        _selectionButtons.Add(button);
        if (tooltip.Contains("撤销", StringComparison.Ordinal) ||
            tooltip.Contains("重做", StringComparison.Ordinal) ||
            tooltip.Contains("清空", StringComparison.Ordinal) ||
            tooltip.Contains("删除", StringComparison.Ordinal))
        {
            _operationButtons.Add(button);
        }
        return button;
    }

    private WpfButton ToolButton(string glyph, string tooltip, AnnotationTool tool)
    {
        var button = NativeVisuals.IconButton(glyph, tooltip, () => ActivateTool(tool), false, 30);
        button.Margin = new Thickness(2, 0, 2, 0);
        _toolButtons.Add((button, tool));
        _selectionButtons.Add(button);
        return button;
    }

    private WpfButton ColorSwatch(string color)
    {
        var button = new WpfButton
        {
            Width = 20,
            Height = 20,
            MinWidth = 20,
            Margin = new Thickness(2, 5, 2, 0),
            Padding = new Thickness(0),
            BorderThickness = new Thickness(1),
            BorderBrush = NativeVisuals.Brush(190, 212, 212, 216),
            Background = new SolidColorBrush(ColorFromHex(color)),
            ToolTip = color,
            Cursor = Cursors.Arrow,
            Focusable = false
        };
        button.Click += (_, _) =>
        {
            _annotationColor = color;
            UpdateToolStates();
        };
        _colorButtons.Add(button);
        return button;
    }

    private static Border Separator()
    {
        return new Border
        {
            Width = 1,
            Height = 20,
            Margin = new Thickness(7, 5, 6, 5),
            Background = NativeVisuals.Brush(120, 212, 212, 216),
            Cursor = Cursors.Arrow
        };
    }

    private void BuildMagnifier()
    {
        _magnifier.Width = 132;
        _magnifier.Padding = new Thickness(8);
        _magnifier.CornerRadius = new CornerRadius(18);
        _magnifier.Background = NativeVisuals.Brush(236, 250, 250, 250);
        _magnifier.BorderBrush = NativeVisuals.Brush(180, 255, 255, 255);
        _magnifier.BorderThickness = new Thickness(1);
        _magnifier.Effect = NativeVisuals.Shadow(58, 24, 8);
        _magnifier.Visibility = Visibility.Collapsed;
        _magnifier.IsHitTestVisible = false;

        _magnifierImage.Width = 96;
        _magnifierImage.Height = 96;
        _magnifierImage.Stretch = Stretch.Fill;
        RenderOptions.SetBitmapScalingMode(_magnifierImage, BitmapScalingMode.NearestNeighbor);

        _magnifierText.FontFamily = new FontFamily("Consolas, Cascadia Mono, Segoe UI");
        _magnifierText.FontSize = 11;
        _magnifierText.FontWeight = FontWeights.SemiBold;
        _magnifierText.Foreground = NativeVisuals.Brush(39, 39, 42);
        _magnifierText.HorizontalAlignment = HorizontalAlignment.Center;
        _magnifierText.Margin = new Thickness(0, 6, 0, 0);

        _magnifier.Child = new StackPanel
        {
            Children =
            {
                new Border
                {
                    Width = 96,
                    Height = 96,
                    CornerRadius = new CornerRadius(48),
                    BorderBrush = NativeVisuals.Brush(31, 41, 51),
                    BorderThickness = new Thickness(1),
                    ClipToBounds = true,
                    Child = _magnifierImage
                },
                _magnifierText
            }
        };
    }

    private void BeginSelection(object sender, MouseButtonEventArgs eventArgs)
    {
        if (IsControlSource(eventArgs.OriginalSource as DependencyObject))
        {
            return;
        }

        CommitTextAnnotation();
        var point = ClampPoint(eventArgs.GetPosition(_overlay));
        _lastPoint = point;
        var selection = SelectionDip();

        if (selection.Width >= 2 && selection.Height >= 2 && selection.Contains(point))
        {
            if (_tool == AnnotationTool.Select && TryBeginMoveAnnotation(point))
            {
                return;
            }
            if (_editMode)
            {
                BeginAnnotation(point);
                return;
            }
        }

        _interactionStart = point;
        _interactionOrigin = selection;
        _resizeAnchor = HitTestResizeAnchor(point, selection);
        if (_resizeAnchor != ResizeAnchor.None)
        {
            _mode = OverlayInteractionMode.Resizing;
        }
        else if (selection.Width >= 2 && selection.Height >= 2 && selection.Contains(point))
        {
            _mode = OverlayInteractionMode.Moving;
        }
        else
        {
            _mode = OverlayInteractionMode.Creating;
            _start = point;
            _end = _start;
            ClearAnnotations();
        }

        _toolbar.Visibility = Visibility.Collapsed;
        _selection.Visibility = Visibility.Visible;
        _magnifier.Visibility = Visibility.Collapsed;
        SetSelectionDragVisuals(true);
        _overlay.CaptureMouse();
        UpdateSelectionVisuals(refreshChrome: false, refreshTools: false, redrawAnnotations: false, layoutToolbar: false);
    }

    private void MoveSelection(object sender, WpfMouseEventArgs eventArgs)
    {
        if (IsControlSource(eventArgs.OriginalSource as DependencyObject))
        {
            Cursor = Cursors.Arrow;
            _magnifier.Visibility = Visibility.Collapsed;
            return;
        }

        var point = ClampPoint(eventArgs.GetPosition(_overlay));
        _lastPoint = point;
        if (_mode == OverlayInteractionMode.None)
        {
            UpdateMagnifier(point);
        }
        else
        {
            _magnifier.Visibility = Visibility.Collapsed;
        }
        switch (_mode)
        {
            case OverlayInteractionMode.None:
                UpdateHoverCursor(point);
                return;
            case OverlayInteractionMode.Creating:
                _end = point;
                break;
            case OverlayInteractionMode.Moving:
                MoveCurrentSelection(point);
                break;
            case OverlayInteractionMode.Resizing:
                ResizeCurrentSelection(point);
                break;
            case OverlayInteractionMode.DrawingAnnotation:
                UpdateDraftAnnotation(point);
                break;
            case OverlayInteractionMode.MovingAnnotation:
                MoveSelectedAnnotation(point);
                break;
        }
        var redrawAnnotations = _mode is OverlayInteractionMode.DrawingAnnotation or OverlayInteractionMode.MovingAnnotation;
        UpdateSelectionVisuals(refreshChrome: false, refreshTools: false, redrawAnnotations: redrawAnnotations, layoutToolbar: false);
    }

    private void AdjustThicknessWithWheel(object sender, MouseWheelEventArgs eventArgs)
    {
        if (_textEditor != null || SelectionDip().Width < 2 || SelectionDip().Height < 2)
        {
            return;
        }
        if (eventArgs.OriginalSource is DependencyObject source && IsControlSource(source) && source is TextBox)
        {
            return;
        }

        var delta = eventArgs.Delta > 0 ? 1 : -1;
        SetThickness(_thickness + delta, true);
        eventArgs.Handled = true;
    }

    private void EndInteraction(object sender, MouseButtonEventArgs eventArgs)
    {
        if (_mode == OverlayInteractionMode.None)
        {
            return;
        }
        if (_mode == OverlayInteractionMode.Creating)
        {
            _end = ClampPoint(eventArgs.GetPosition(_overlay));
        }
        else if (_mode == OverlayInteractionMode.DrawingAnnotation)
        {
            CommitDraftAnnotation();
        }
        _mode = OverlayInteractionMode.None;
        _resizeAnchor = ResizeAnchor.None;
        _annotationMoveOrigin = null;
        SetSelectionDragVisuals(false);
        _overlay.ReleaseMouseCapture();
        UpdateSelectionVisuals();
    }

    private void HandleKeyDown(object sender, WpfKeyEventArgs eventArgs)
    {
        if (_textEditor != null)
        {
            if (eventArgs.Key == Key.Escape)
            {
                CancelTextAnnotation();
                eventArgs.Handled = true;
            }
            return;
        }

        var control = (Keyboard.Modifiers & ModifierKeys.Control) != 0;
        var shift = (Keyboard.Modifiers & ModifierKeys.Shift) != 0;
        if (eventArgs.Key == Key.Escape)
        {
            Complete(new CaptureResponse { Ok = false, Canceled = true, Message = "已取消截图" });
            eventArgs.Handled = true;
            return;
        }
        var colorShortcut = ColorPickerShortcutResolver.Resolve(
            isShiftKey: eventArgs.Key is Key.LeftShift or Key.RightShift,
            isCKey: eventArgs.Key == Key.C,
            controlDown: control,
            isRepeat: eventArgs.IsRepeat);
        if (colorShortcut == ColorPickerShortcutAction.ToggleFormat)
        {
            ToggleColorFormat();
            eventArgs.Handled = true;
            return;
        }
        if (colorShortcut == ColorPickerShortcutAction.CopyColor)
        {
            CopyPointerColor();
            eventArgs.Handled = true;
            return;
        }
        if (colorShortcut == ColorPickerShortcutAction.Consume)
        {
            eventArgs.Handled = true;
            return;
        }
        if (control && shift && eventArgs.Key == Key.Z)
        {
            RedoAnnotation();
            eventArgs.Handled = true;
            return;
        }
        if (control && eventArgs.Key == Key.Z)
        {
            UndoAnnotation();
            eventArgs.Handled = true;
            return;
        }
        if (control && eventArgs.Key == Key.Y)
        {
            RedoAnnotation();
            eventArgs.Handled = true;
            return;
        }
        if (control && eventArgs.Key == Key.S)
        {
            Finish("save_as");
            eventArgs.Handled = true;
            return;
        }
        if (SelectionDip().Width < 2 || SelectionDip().Height < 2)
        {
            return;
        }
        if (shift && eventArgs.Key == Key.Enter)
        {
            Finish("redact_copy");
            eventArgs.Handled = true;
            return;
        }
        if (eventArgs.Key == Key.Enter)
        {
            Finish("copy");
            eventArgs.Handled = true;
            return;
        }
        if (eventArgs.Key == Key.P)
        {
            Finish("pin");
            eventArgs.Handled = true;
            return;
        }
        if (eventArgs.Key == Key.Q)
        {
            Finish("qr");
            eventArgs.Handled = true;
            return;
        }
        if (eventArgs.Key is Key.Delete or Key.Back && _selectedOperationIndex >= 0)
        {
            DeleteSelectedAnnotation();
            eventArgs.Handled = true;
            return;
        }
        if (eventArgs.Key == Key.Back && _operations.Count > 0)
        {
            UndoAnnotation();
            eventArgs.Handled = true;
            return;
        }

        var tool = eventArgs.Key switch
        {
            Key.R => AnnotationTool.Rect,
            Key.L => AnnotationTool.Line,
            Key.A => AnnotationTool.Arrow,
            Key.B => AnnotationTool.Pen,
            Key.H => AnnotationTool.Highlight,
            Key.M => AnnotationTool.Mosaic,
            Key.T => AnnotationTool.Text,
            Key.N => AnnotationTool.Number,
            Key.E => AnnotationTool.Eraser,
            Key.V => AnnotationTool.Select,
            _ => (AnnotationTool?)null
        };
        if (tool.HasValue)
        {
            ActivateTool(tool.Value);
            eventArgs.Handled = true;
        }
    }

    private void ToggleColorFormat()
    {
        var nextFormat = ColorFormatPreferences.Current == ColorFormat.Rgb ? ColorFormat.Hex : ColorFormat.Rgb;
        var saved = ColorFormatPreferences.TrySetCurrent(nextFormat);
        RefreshPointerColorText();
        ShowFeedback("取色格式: " + (nextFormat == ColorFormat.Rgb ? "RGB" : "HEX") + (saved ? "" : "（未保存）"));
    }

    private void UpdateSelectionVisuals(bool refreshChrome = true, bool refreshTools = true, bool redrawAnnotations = true, bool layoutToolbar = true)
    {
        var bounds = SelectionDip();
        var totalWidth = Math.Max(ActualWidth, Width);
        var totalHeight = Math.Max(ActualHeight, Height);
        if (refreshChrome)
        {
            PositionOverlayChrome();
        }
        if (refreshTools)
        {
            UpdateToolStates();
        }

        if (bounds.Width < 1 || bounds.Height < 1)
        {
            _selection.Visibility = Visibility.Collapsed;
            _toolbar.Visibility = Visibility.Collapsed;
            _selectionSize.Visibility = Visibility.Collapsed;
            _annotationLayer.Visibility = Visibility.Collapsed;
            _magnifier.Visibility = Visibility.Collapsed;
            foreach (var handle in _handles)
            {
                handle.Element.Visibility = Visibility.Collapsed;
            }
            SetRect(_topMask, 0, 0, totalWidth, totalHeight);
            SetRect(_leftMask, 0, 0, 0, 0);
            SetRect(_rightMask, 0, 0, 0, 0);
            SetRect(_bottomMask, 0, 0, 0, 0);
            return;
        }

        _selection.Visibility = Visibility.Visible;
        SetElement(_selection, bounds.X, bounds.Y, bounds.Width, bounds.Height);
        if (HasAnnotationVisuals())
        {
            SetElement(_annotationLayer, bounds.X, bounds.Y, bounds.Width, bounds.Height);
            _annotationLayer.Visibility = Visibility.Visible;
            if (redrawAnnotations)
            {
                RedrawAnnotations();
            }
        }
        else
        {
            _annotationLayer.Visibility = Visibility.Collapsed;
            if (_annotationLayer.Children.Count > 0)
            {
                _annotationLayer.Children.Clear();
            }
        }
        PositionSelectionSize(bounds, totalWidth, totalHeight);
        PositionHandles(bounds);
        SetRect(_topMask, 0, 0, totalWidth, bounds.Y);
        SetRect(_leftMask, 0, bounds.Y, bounds.X, bounds.Height);
        SetRect(_rightMask, bounds.X + bounds.Width, bounds.Y, Math.Max(0, totalWidth - bounds.X - bounds.Width), bounds.Height);
        SetRect(_bottomMask, 0, bounds.Y + bounds.Height, totalWidth, Math.Max(0, totalHeight - bounds.Y - bounds.Height));

        if (!layoutToolbar)
        {
            return;
        }

        if ((_mode is OverlayInteractionMode.None or OverlayInteractionMode.DrawingAnnotation) && bounds.Width >= 2 && bounds.Height >= 2)
        {
            _toolbar.Visibility = Visibility.Visible;
            var maxToolbarWidth = Math.Min(980, Math.Max(260, totalWidth - 24));
            _primaryToolbarRow.MaxWidth = maxToolbarWidth - 12;
            _secondaryToolbarRow.MaxWidth = maxToolbarWidth - 12;
            _toolbar.Measure(new WpfSize(maxToolbarWidth, double.PositiveInfinity));
            var toolbarWidth = Math.Min(maxToolbarWidth, _toolbar.DesiredSize.Width);
            var toolbarHeight = _toolbar.DesiredSize.Height;
            var x = Math.Clamp(bounds.X + bounds.Width - toolbarWidth, 8, Math.Max(8, totalWidth - toolbarWidth - 8));
            var y = bounds.Y + bounds.Height + 8;
            if (y + toolbarHeight > totalHeight - 8)
            {
                y = Math.Max(8, bounds.Y - toolbarHeight - 8);
            }
            SetElement(_toolbar, x, y, toolbarWidth, toolbarHeight);
        }
        else
        {
            _toolbar.Visibility = Visibility.Collapsed;
        }
    }

    private void PositionOverlayChrome()
    {
        var totalWidth = Math.Max(ActualWidth, Width);
        var totalHeight = Math.Max(ActualHeight, Height);
        if (_closeButton != null)
        {
            SetElement(_closeButton, Math.Max(12, totalWidth - 46), 14, 32, 32);
        }

        _hint.Measure(new WpfSize(double.PositiveInfinity, double.PositiveInfinity));
        var hintWidth = Math.Min(_hint.DesiredSize.Width, Math.Max(260, totalWidth - 24));
        var hintHeight = _hint.DesiredSize.Height;
        SetElement(_hint, Math.Max(12, (totalWidth - hintWidth) / 2), Math.Max(12, totalHeight - hintHeight - 18), hintWidth, hintHeight);

        _feedback.Measure(new WpfSize(double.PositiveInfinity, double.PositiveInfinity));
        var feedbackWidth = _feedback.DesiredSize.Width;
        var feedbackHeight = _feedback.DesiredSize.Height;
        SetElement(_feedback, Math.Max(12, (totalWidth - feedbackWidth) / 2), 18, feedbackWidth, feedbackHeight);
    }

    private void PositionSelectionSize(Rect bounds, double totalWidth, double totalHeight)
    {
        var physical = SelectionPhysical();
        var text = $"{physical.Width} x {physical.Height}";
        if (!string.Equals(_lastSelectionSizeText, text, StringComparison.Ordinal))
        {
            _lastSelectionSizeText = text;
            _selectionSizeText.Text = text;
            if (_lastSelectionSizeTextLength != text.Length)
            {
                _lastSelectionSizeTextLength = text.Length;
                _selectionSize.Measure(new WpfSize(double.PositiveInfinity, double.PositiveInfinity));
                _lastSelectionSizeMeasure = _selectionSize.DesiredSize;
            }
        }
        _selectionSize.Visibility = Visibility.Visible;
        if (_lastSelectionSizeMeasure.IsEmpty)
        {
            _selectionSize.Measure(new WpfSize(double.PositiveInfinity, double.PositiveInfinity));
            _lastSelectionSizeMeasure = _selectionSize.DesiredSize;
        }
        var width = _lastSelectionSizeMeasure.Width;
        var height = _lastSelectionSizeMeasure.Height;
        var x = Math.Clamp(bounds.Right - width, 8, Math.Max(8, totalWidth - width - 8));
        var y = bounds.Y - height - 7;
        if (y < 8)
        {
            y = Math.Min(totalHeight - height - 8, bounds.Y + 7);
        }
        SetElement(_selectionSize, x, Math.Max(8, y), width, height);
    }

    private void PositionHandles(Rect bounds)
    {
        foreach (var handle in _handles)
        {
            if (_mode is OverlayInteractionMode.Creating or OverlayInteractionMode.DrawingAnnotation || bounds.Width < 10 || bounds.Height < 10)
            {
                handle.Element.Visibility = Visibility.Collapsed;
                continue;
            }
            var center = HandleCenter(bounds, handle.Anchor);
            var size = handle.Element.Width;
            SetElement(handle.Element, center.X - size / 2, center.Y - size / 2, size, size);
            handle.Element.Visibility = Visibility.Visible;
        }
    }

    private IReadOnlyList<(Border Element, ResizeAnchor Anchor)> BuildHandles()
    {
        var anchors = new[]
        {
            ResizeAnchor.TopLeft,
            ResizeAnchor.Top,
            ResizeAnchor.TopRight,
            ResizeAnchor.Right,
            ResizeAnchor.BottomRight,
            ResizeAnchor.Bottom,
            ResizeAnchor.BottomLeft,
            ResizeAnchor.Left
        };
        return anchors.Select(anchor =>
        {
            var handle = new Border
            {
                Width = 9,
                Height = 9,
                CornerRadius = new CornerRadius(4.5),
                Background = NativeVisuals.Brush(250, 250, 250, 250),
                BorderBrush = NativeVisuals.Brush(220, 31, 41, 51),
                BorderThickness = new Thickness(1),
                Visibility = Visibility.Collapsed,
                IsHitTestVisible = false,
                Effect = NativeVisuals.Shadow(44, 10, 2)
            };
            return (handle, anchor);
        }).ToList();
    }

    private void MoveCurrentSelection(WpfPoint point)
    {
        var totalWidth = Math.Max(ActualWidth, Width);
        var totalHeight = Math.Max(ActualHeight, Height);
        var dx = point.X - _interactionStart.X;
        var dy = point.Y - _interactionStart.Y;
        var x = Math.Clamp(_interactionOrigin.X + dx, 0, Math.Max(0, totalWidth - _interactionOrigin.Width));
        var y = Math.Clamp(_interactionOrigin.Y + dy, 0, Math.Max(0, totalHeight - _interactionOrigin.Height));
        _start = new WpfPoint(x, y);
        _end = new WpfPoint(x + _interactionOrigin.Width, y + _interactionOrigin.Height);
    }

    private void ResizeCurrentSelection(WpfPoint point)
    {
        var left = _interactionOrigin.Left;
        var top = _interactionOrigin.Top;
        var right = _interactionOrigin.Right;
        var bottom = _interactionOrigin.Bottom;

        switch (_resizeAnchor)
        {
            case ResizeAnchor.TopLeft:
                left = point.X;
                top = point.Y;
                break;
            case ResizeAnchor.Top:
                top = point.Y;
                break;
            case ResizeAnchor.TopRight:
                right = point.X;
                top = point.Y;
                break;
            case ResizeAnchor.Right:
                right = point.X;
                break;
            case ResizeAnchor.BottomRight:
                right = point.X;
                bottom = point.Y;
                break;
            case ResizeAnchor.Bottom:
                bottom = point.Y;
                break;
            case ResizeAnchor.BottomLeft:
                left = point.X;
                bottom = point.Y;
                break;
            case ResizeAnchor.Left:
                left = point.X;
                break;
        }

        _start = new WpfPoint(left, top);
        _end = new WpfPoint(right, bottom);
    }

    private void UpdateHoverCursor(WpfPoint point)
    {
        var selection = SelectionDip();
        var anchor = HitTestResizeAnchor(point, selection);
        if (_editMode && selection.Contains(point))
        {
            Cursor = _tool == AnnotationTool.Text ? Cursors.IBeam : WpfCursors.Pen;
        }
        else if (_tool == AnnotationTool.Select && selection.Contains(point))
        {
            Cursor = Cursors.Hand;
        }
        else if (anchor != ResizeAnchor.None)
        {
            Cursor = CursorForAnchor(anchor);
        }
        else if (selection.Width >= 2 && selection.Height >= 2 && selection.Contains(point))
        {
            Cursor = Cursors.SizeAll;
        }
        else
        {
            Cursor = WpfCursors.Cross;
        }
    }

    private ResizeAnchor HitTestResizeAnchor(WpfPoint point, Rect bounds)
    {
        if (bounds.Width < 10 || bounds.Height < 10)
        {
            return ResizeAnchor.None;
        }
        const double hitSize = 14;
        foreach (var handle in _handles)
        {
            var center = HandleCenter(bounds, handle.Anchor);
            var hitRect = new Rect(center.X - hitSize / 2, center.Y - hitSize / 2, hitSize, hitSize);
            if (hitRect.Contains(point))
            {
                return handle.Anchor;
            }
        }
        return ResizeAnchor.None;
    }

    private static WpfPoint HandleCenter(Rect bounds, ResizeAnchor anchor)
    {
        return anchor switch
        {
            ResizeAnchor.TopLeft => new WpfPoint(bounds.Left, bounds.Top),
            ResizeAnchor.Top => new WpfPoint(bounds.Left + bounds.Width / 2, bounds.Top),
            ResizeAnchor.TopRight => new WpfPoint(bounds.Right, bounds.Top),
            ResizeAnchor.Right => new WpfPoint(bounds.Right, bounds.Top + bounds.Height / 2),
            ResizeAnchor.BottomRight => new WpfPoint(bounds.Right, bounds.Bottom),
            ResizeAnchor.Bottom => new WpfPoint(bounds.Left + bounds.Width / 2, bounds.Bottom),
            ResizeAnchor.BottomLeft => new WpfPoint(bounds.Left, bounds.Bottom),
            ResizeAnchor.Left => new WpfPoint(bounds.Left, bounds.Top + bounds.Height / 2),
            _ => new WpfPoint(bounds.Left, bounds.Top)
        };
    }

    private static Cursor CursorForAnchor(ResizeAnchor anchor)
    {
        return anchor switch
        {
            ResizeAnchor.TopLeft or ResizeAnchor.BottomRight => Cursors.SizeNWSE,
            ResizeAnchor.TopRight or ResizeAnchor.BottomLeft => Cursors.SizeNESW,
            ResizeAnchor.Top or ResizeAnchor.Bottom => Cursors.SizeNS,
            ResizeAnchor.Left or ResizeAnchor.Right => Cursors.SizeWE,
            _ => WpfCursors.Cross
        };
    }

    private void ActivateTool(AnnotationTool tool)
    {
        if (SelectionDip().Width < 2 || SelectionDip().Height < 2)
        {
            ShowFeedback("先拖拽选择区域");
            return;
        }
        CommitTextAnnotation();
        var active = tool == _tool && (tool == AnnotationTool.Select || _editMode);
        if (active)
        {
            _tool = AnnotationTool.None;
            _editMode = false;
            _selectedOperationIndex = -1;
            _draftOperation = null;
            ShowFeedback("标注已关闭");
            UpdateToolStates();
            UpdateSelectionVisuals();
            return;
        }

        _tool = tool;
        _editMode = tool != AnnotationTool.Select;
        _selectedOperationIndex = -1;
        _draftOperation = null;
        ShowFeedback(ToolLabel(tool));
        UpdateToolStates();
        UpdateSelectionVisuals();
    }

    private void BeginAnnotation(WpfPoint screenPoint)
    {
        var local = ToSelectionPoint(screenPoint);
        if (_tool == AnnotationTool.Text)
        {
            StartTextAnnotation(local);
            return;
        }
        if (_tool == AnnotationTool.Number)
        {
            AddNumberAnnotation(local);
            return;
        }
        _annotationStart = local;
        _draftOperation = CreateAnnotation(local, local);
        _mode = OverlayInteractionMode.DrawingAnnotation;
        _overlay.CaptureMouse();
        RedrawAnnotations();
    }

    private void UpdateDraftAnnotation(WpfPoint screenPoint)
    {
        var local = ToSelectionPoint(screenPoint);
        if (_draftOperation == null)
        {
            return;
        }
        if (IsPathTool(_tool))
        {
            AppendPoint(_draftOperation, local);
        }
        else
        {
            _draftOperation = CreateAnnotation(_annotationStart, local);
        }
        RedrawAnnotations();
    }

    private void CommitDraftAnnotation()
    {
        if (_draftOperation == null)
        {
            return;
        }
        var operation = CloneOperation(_draftOperation);
        _draftOperation = null;
        if (!IsUsefulAnnotation(operation))
        {
            ShowFeedback("标注区域太小");
            return;
        }
        if (NormalizeKind(operation.Kind) == "eraser")
        {
            if (!ApplyEraser(operation))
            {
                ShowFeedback("没有可擦除标注");
                return;
            }
            _redoOperations.Clear();
            _selectedOperationIndex = -1;
            RedrawAnnotations();
            UpdateToolStates();
            return;
        }
        _operations.Add(operation);
        _redoOperations.Clear();
        _selectedOperationIndex = _operations.Count - 1;
        RedrawAnnotations();
    }

    private bool TryBeginMoveAnnotation(WpfPoint screenPoint)
    {
        var local = ToSelectionPoint(screenPoint);
        for (var index = _operations.Count - 1; index >= 0; index--)
        {
            if (!OperationBounds(_operations[index]).InflateBy(8).Contains(local))
            {
                continue;
            }
            _selectedOperationIndex = index;
            _annotationMoveStart = local;
            _annotationMoveOrigin = CloneOperation(_operations[index]);
            _mode = OverlayInteractionMode.MovingAnnotation;
            _overlay.CaptureMouse();
            RedrawAnnotations();
            return true;
        }
        _selectedOperationIndex = -1;
        RedrawAnnotations();
        return false;
    }

    private void MoveSelectedAnnotation(WpfPoint screenPoint)
    {
        if (_selectedOperationIndex < 0 || _selectedOperationIndex >= _operations.Count || _annotationMoveOrigin == null)
        {
            return;
        }
        var local = ToSelectionPoint(screenPoint);
        var dx = (int)Math.Round(local.X - _annotationMoveStart.X);
        var dy = (int)Math.Round(local.Y - _annotationMoveStart.Y);
        _operations[_selectedOperationIndex] = TranslateOperation(_annotationMoveOrigin, dx, dy);
        RedrawAnnotations();
    }

    private void StartTextAnnotation(WpfPoint local)
    {
        CancelTextAnnotation();
        _textEditorPoint = local;
        _textEditor = new TextBox
        {
            Width = Math.Max(140, SelectionDip().Width - local.X - 16),
            MinHeight = 34,
            MaxHeight = 160,
            AcceptsReturn = true,
            TextWrapping = TextWrapping.Wrap,
            FontSize = Math.Max(12, _thickness * 4 + 8),
            FontFamily = new FontFamily("Microsoft YaHei UI, Segoe UI"),
            Foreground = new SolidColorBrush(ColorFromHex(_annotationColor)),
            Background = NativeVisuals.Brush(245, 255, 255, 255),
            BorderBrush = NativeVisuals.Brush(31, 41, 51),
            BorderThickness = new Thickness(1),
            Padding = new Thickness(6)
        };
        _textEditor.KeyDown += (_, args) =>
        {
            if (args.Key == Key.Escape)
            {
                CancelTextAnnotation();
                args.Handled = true;
            }
            else if (args.Key == Key.Enter && (Keyboard.Modifiers & ModifierKeys.Control) != 0)
            {
                CommitTextAnnotation();
                args.Handled = true;
            }
        };
        _textEditor.LostFocus += (_, _) => CommitTextAnnotation();
        _textEditorHost = new Border
        {
            Child = _textEditor,
            Effect = NativeVisuals.Shadow(36, 12, 3)
        };
        RedrawAnnotations();
        _textEditor.Focus();
        Keyboard.Focus(_textEditor);
        _ = Dispatcher.BeginInvoke(new Action(() =>
        {
            if (_textEditor == null)
            {
                return;
            }
            _textEditor.Focus();
            Keyboard.Focus(_textEditor);
        }), System.Windows.Threading.DispatcherPriority.Input);
    }

    private void CommitTextAnnotation()
    {
        if (_textEditor == null)
        {
            return;
        }
        var text = _textEditor.Text.Trim();
        var width = (int)Math.Round(_textEditor.ActualWidth > 0 ? _textEditor.ActualWidth : _textEditor.Width);
        var fontSize = Math.Max(12, (int)Math.Round(_textEditor.FontSize));
        CancelTextAnnotation();
        if (text.Length == 0)
        {
            return;
        }
        _operations.Add(new AnnotationOperation
        {
            Kind = "text",
            X = (int)Math.Round(_textEditorPoint.X),
            Y = (int)Math.Round(_textEditorPoint.Y),
            Width = width,
            Color = _annotationColor,
            FontSize = fontSize,
            Text = text
        });
        _redoOperations.Clear();
        _selectedOperationIndex = _operations.Count - 1;
        RedrawAnnotations();
    }

    private void CancelTextAnnotation()
    {
        if (_textEditorHost != null)
        {
            _annotationLayer.Children.Remove(_textEditorHost);
        }
        _textEditorHost = null;
        _textEditor = null;
    }

    private void AddNumberAnnotation(WpfPoint local)
    {
        _operations.Add(new AnnotationOperation
        {
            Kind = "number",
            X = (int)Math.Round(local.X),
            Y = (int)Math.Round(local.Y),
            Color = _annotationColor,
            StrokeWidth = _thickness,
            FontSize = Math.Max(14, _thickness * 4 + 10),
            Number = _numberCounter++
        });
        _redoOperations.Clear();
        _selectedOperationIndex = _operations.Count - 1;
        RedrawAnnotations();
    }

    private void UndoAnnotation()
    {
        CommitTextAnnotation();
        if (_operations.Count == 0)
        {
            ShowFeedback("没有可撤销的标注");
            return;
        }
        var last = _operations[^1];
        _operations.RemoveAt(_operations.Count - 1);
        _redoOperations.Add(last);
        _selectedOperationIndex = -1;
        RedrawAnnotations();
        UpdateToolStates();
    }

    private void RedoAnnotation()
    {
        if (_redoOperations.Count == 0)
        {
            ShowFeedback("没有可重做的标注");
            return;
        }
        var operation = _redoOperations[^1];
        _redoOperations.RemoveAt(_redoOperations.Count - 1);
        _operations.Add(operation);
        _selectedOperationIndex = _operations.Count - 1;
        RedrawAnnotations();
        UpdateToolStates();
    }

    private void ClearAnnotations()
    {
        CancelTextAnnotation();
        _operations.Clear();
        _redoOperations.Clear();
        _draftOperation = null;
        _selectedOperationIndex = -1;
        RedrawAnnotations();
        UpdateToolStates();
    }

    private void DeleteSelectedAnnotation()
    {
        if (_selectedOperationIndex < 0 || _selectedOperationIndex >= _operations.Count)
        {
            ShowFeedback("先选择标注");
            return;
        }
        _redoOperations.Add(_operations[_selectedOperationIndex]);
        _operations.RemoveAt(_selectedOperationIndex);
        _selectedOperationIndex = -1;
        RedrawAnnotations();
        UpdateToolStates();
    }

    private void ResetSelection()
    {
        CancelTextAnnotation();
        _start = new WpfPoint();
        _end = new WpfPoint();
        _mode = OverlayInteractionMode.None;
        ClearAnnotations();
        UpdateSelectionVisuals();
    }

    private void RedrawAnnotations()
    {
        var editor = _textEditorHost;
        if (!HasAnnotationVisuals())
        {
            if (_annotationLayer.Children.Count > 0)
            {
                _annotationLayer.Children.Clear();
            }
            return;
        }

        EnsureAnnotationLayerVisible();
        _annotationLayer.Children.Clear();
        foreach (var operation in _operations)
        {
            AddOperationVisual(operation);
        }
        if (_draftOperation != null)
        {
            AddOperationVisual(_draftOperation, true);
        }
        if (_selectedOperationIndex >= 0 && _selectedOperationIndex < _operations.Count)
        {
            AddSelectionOutline(OperationBounds(_operations[_selectedOperationIndex]));
        }
        if (editor != null)
        {
            _annotationLayer.Children.Add(editor);
            Canvas.SetLeft(editor, _textEditorPoint.X);
            Canvas.SetTop(editor, _textEditorPoint.Y);
        }
    }

    private void EnsureAnnotationLayerVisible()
    {
        var bounds = SelectionDip();
        if (bounds.Width < 1 || bounds.Height < 1)
        {
            return;
        }
        SetElement(_annotationLayer, bounds.X, bounds.Y, bounds.Width, bounds.Height);
        _annotationLayer.Visibility = Visibility.Visible;
    }

    private bool HasAnnotationVisuals()
    {
        return _operations.Count > 0 || _draftOperation != null || _selectedOperationIndex >= 0 || _textEditorHost != null;
    }

    private void AddOperationVisual(AnnotationOperation operation, bool draft = false)
    {
        var color = ColorFromHex(string.IsNullOrWhiteSpace(operation.Color) ? _annotationColor : operation.Color);
        var brush = new SolidColorBrush(color);
        var stroke = Math.Max(1, operation.StrokeWidth == 0 ? _thickness : operation.StrokeWidth);
        var opacity = draft ? 0.72 : 1.0;
        switch (NormalizeKind(operation.Kind))
        {
            case "rect":
                var rect = NormalizeRect(operation);
                var rectangle = new WpfRectangle
                {
                    Width = rect.Width,
                    Height = rect.Height,
                    Stroke = brush,
                    StrokeThickness = stroke,
                    Fill = WpfBrushes.Transparent,
                    Opacity = opacity
                };
                _annotationLayer.Children.Add(rectangle);
                Canvas.SetLeft(rectangle, rect.X);
                Canvas.SetTop(rectangle, rect.Y);
                break;
            case "line":
                AddLineVisual(operation.X, operation.Y, operation.EndX, operation.EndY, brush, stroke, opacity);
                break;
            case "arrow":
                AddArrowVisual(operation, brush, stroke, opacity);
                break;
            case "pen":
            case "highlight":
                AddPathVisual(operation, brush, stroke, opacity);
                break;
            case "eraser":
                AddEraserPreviewVisual(operation, stroke, opacity);
                break;
            case "mosaic":
                AddMosaicVisual(operation, opacity);
                break;
            case "text":
                var text = new TextBlock
                {
                    Text = operation.Text,
                    Foreground = brush,
                    FontFamily = new FontFamily("Microsoft YaHei UI, Segoe UI"),
                    FontSize = Math.Max(12, operation.FontSize == 0 ? 20 : operation.FontSize),
                    FontWeight = FontWeights.SemiBold,
                    TextWrapping = TextWrapping.Wrap,
                    Width = Math.Max(48, operation.Width == 0 ? 220 : operation.Width),
                    Opacity = opacity
                };
                _annotationLayer.Children.Add(text);
                Canvas.SetLeft(text, operation.X);
                Canvas.SetTop(text, operation.Y);
                break;
            case "number":
                var size = Math.Max(22, operation.FontSize == 0 ? 26 : operation.FontSize + 8);
                var marker = new Border
                {
                    Width = size,
                    Height = size,
                    CornerRadius = new CornerRadius(size / 2),
                    Background = NativeVisuals.Brush(238, 255, 255, 255),
                    BorderBrush = brush,
                    BorderThickness = new Thickness(Math.Max(1, stroke)),
                    Opacity = opacity,
                    Child = new TextBlock
                    {
                        Text = Math.Max(1, operation.Number).ToString(CultureInfo.InvariantCulture),
                        Foreground = brush,
                        FontWeight = FontWeights.Bold,
                        HorizontalAlignment = HorizontalAlignment.Center,
                        VerticalAlignment = VerticalAlignment.Center
                    }
                };
                _annotationLayer.Children.Add(marker);
                Canvas.SetLeft(marker, operation.X - size / 2);
                Canvas.SetTop(marker, operation.Y - size / 2);
                break;
        }
    }

    private void AddLineVisual(double x1, double y1, double x2, double y2, Brush brush, double stroke, double opacity)
    {
        var line = new Line
        {
            X1 = x1,
            Y1 = y1,
            X2 = x2,
            Y2 = y2,
            Stroke = brush,
            StrokeThickness = stroke,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round,
            Opacity = opacity
        };
        _annotationLayer.Children.Add(line);
    }

    private void AddArrowVisual(AnnotationOperation operation, Brush brush, double stroke, double opacity)
    {
        AddLineVisual(operation.X, operation.Y, operation.EndX, operation.EndY, brush, stroke, opacity);
        foreach (var point in ArrowHead(operation.X, operation.Y, operation.EndX, operation.EndY, stroke))
        {
            AddLineVisual(operation.EndX, operation.EndY, point.X, point.Y, brush, stroke, opacity);
        }
    }

    private void AddPathVisual(AnnotationOperation operation, Brush brush, double stroke, double opacity)
    {
        var points = operation.Points ?? [];
        if (points.Count == 0)
        {
            return;
        }
        var polyline = new Polyline
        {
            Stroke = brush,
            StrokeThickness = NormalizeKind(operation.Kind) == "highlight" ? Math.Max(10, stroke * 4) : stroke,
            StrokeLineJoin = PenLineJoin.Round,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round,
            Opacity = NormalizeKind(operation.Kind) == "highlight" ? 0.42 : opacity
        };
        foreach (var point in points)
        {
            polyline.Points.Add(new WpfPoint(point.X, point.Y));
        }
        _annotationLayer.Children.Add(polyline);
    }

    private void AddEraserPreviewVisual(AnnotationOperation operation, double stroke, double opacity)
    {
        var points = operation.Points ?? [];
        if (points.Count == 0)
        {
            return;
        }
        var preview = new Polyline
        {
            Stroke = NativeVisuals.Brush(180, 31, 41, 51),
            StrokeThickness = Math.Max(8, stroke * 4),
            StrokeLineJoin = PenLineJoin.Round,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round,
            StrokeDashArray = [3, 3],
            Opacity = Math.Min(0.45, opacity)
        };
        foreach (var point in points)
        {
            preview.Points.Add(new WpfPoint(point.X, point.Y));
        }
        _annotationLayer.Children.Add(preview);
    }

    private void AddMosaicVisual(AnnotationOperation operation, double opacity)
    {
        var block = Math.Max(6, operation.PixelSize == 0 ? _thickness * 4 : operation.PixelSize);
        if (operation.Points is { Count: > 1 } points)
        {
            AddMosaicPreviewPath(points, block, opacity);
            return;
        }
        AddMosaicPreviewRect(NormalizeRect(operation), block, opacity);
    }

    private void AddMosaicPreviewPath(IReadOnlyList<AnnotationPoint> points, int block, double opacity)
    {
        var selection = SelectionDip();
        var selectionBounds = new Rect(0, 0, selection.Width, selection.Height);
        var rects = new List<Rect>();
        var union = Rect.Empty;
        foreach (var point in points)
        {
            var rect = new Rect(point.X - block, point.Y - block, block * 2, block * 2);
            rect.Intersect(selectionBounds);
            if (rect.Width <= 0 || rect.Height <= 0)
            {
                continue;
            }
            rects.Add(rect);
            union = union.IsEmpty ? rect : Rect.Union(union, rect);
        }
        if (rects.Count == 0 || union.IsEmpty)
        {
            return;
        }

        var clip = new GeometryGroup { FillRule = FillRule.Nonzero };
        foreach (var rect in rects)
        {
            clip.Children.Add(new RectangleGeometry(new Rect(rect.X - union.X, rect.Y - union.Y, rect.Width, rect.Height)));
        }
        clip.Freeze();
        AddMosaicPreviewRect(union, block, opacity, clip, false);
    }

    private void AddMosaicPreviewRect(Rect localRect, int block, double opacity, Geometry? clip = null, bool showBorder = true)
    {
        var selection = SelectionDip();
        localRect.Intersect(new Rect(0, 0, selection.Width, selection.Height));
        if (localRect.Width <= 0 || localRect.Height <= 0)
        {
            return;
        }

        var physicalSelection = SelectionPhysical();
        var scaleX = physicalSelection.Width / Math.Max(1, selection.Width);
        var scaleY = physicalSelection.Height / Math.Max(1, selection.Height);
        var sourceRect = new Int32Rect(
            physicalSelection.X + (int)Math.Floor(localRect.X * scaleX),
            physicalSelection.Y + (int)Math.Floor(localRect.Y * scaleY),
            Math.Max(1, (int)Math.Ceiling(localRect.Width * scaleX)),
            Math.Max(1, (int)Math.Ceiling(localRect.Height * scaleY)));
        var source = _capture.CropSource(sourceRect);
        var lowWidth = Math.Max(1, (int)Math.Ceiling(localRect.Width / Math.Max(1, block)));
        var lowHeight = Math.Max(1, (int)Math.Ceiling(localRect.Height / Math.Max(1, block)));
        var low = new TransformedBitmap(source, new ScaleTransform(lowWidth / Math.Max(1.0, source.PixelWidth), lowHeight / Math.Max(1.0, source.PixelHeight)));
        low.Freeze();

        var preview = new Border
        {
            Width = localRect.Width,
            Height = localRect.Height,
            Opacity = opacity,
            BorderBrush = showBorder ? NativeVisuals.Brush(184, 31, 41, 51) : WpfBrushes.Transparent,
            BorderThickness = showBorder ? new Thickness(1) : new Thickness(0),
            ClipToBounds = true,
            Clip = clip,
            Child = new WpfImage
            {
                Source = low,
                Width = localRect.Width,
                Height = localRect.Height,
                Stretch = Stretch.Fill
            }
        };
        RenderOptions.SetBitmapScalingMode(preview, BitmapScalingMode.NearestNeighbor);
        _annotationLayer.Children.Add(preview);
        Canvas.SetLeft(preview, localRect.X);
        Canvas.SetTop(preview, localRect.Y);
    }

    private void AddSelectionOutline(Rect rect)
    {
        if (rect.IsEmpty)
        {
            return;
        }
        var outline = new WpfRectangle
        {
            Width = Math.Max(1, rect.Width),
            Height = Math.Max(1, rect.Height),
            Stroke = NativeVisuals.Brush(31, 41, 51),
            StrokeThickness = 1,
            StrokeDashArray = [4, 3],
            Fill = WpfBrushes.Transparent,
            IsHitTestVisible = false
        };
        _annotationLayer.Children.Add(outline);
        Canvas.SetLeft(outline, rect.X);
        Canvas.SetTop(outline, rect.Y);
    }

    private void Finish(string action)
    {
        if (action == "redact_copy")
        {
            _ = FinishRedactCopyAsync();
            return;
        }

        var totalStarted = Environment.TickCount64;
        if (_ocrBusy || _redactBusy)
        {
            ShowPersistentFeedback(_redactBusy ? "正在打码并复制" : "OCR 识别中");
            return;
        }

        CommitTextAnnotation();
        var physical = SelectionPhysical();
        var selection = SelectionDip();
        if (physical.Width < 2 || physical.Height < 2 || selection.Width < 2 || selection.Height < 2)
        {
            ShowFeedback("先拖拽选择区域");
            return;
        }

        var savedPath = "";
        if (action == "save_as")
        {
            var dialog = new WpfSaveFileDialog
            {
                Title = "保存截图",
                Filter = "PNG 图片|*.png",
                FileName = "Ariadne_" + DateTime.Now.ToString("yyyyMMdd_HHmmss", CultureInfo.InvariantCulture) + ".png",
                AddExtension = true,
                DefaultExt = ".png"
            };
            if (dialog.ShowDialog(this) != true)
            {
                ShowFeedback("已取消另存");
                return;
            }
            savedPath = dialog.FileName;
        }

        byte[] png;
        long renderMs;
        try
        {
            var renderStarted = Environment.TickCount64;
            png = RenderSelectionPng(physical, selection);
            renderMs = Environment.TickCount64 - renderStarted;
        }
        catch (Exception ex)
        {
            Complete(new CaptureResponse { Ok = false, Message = "截图失败: " + ex.Message });
            return;
        }

        var clipboardWritten = false;
        long clipboardMs = 0;
        if (action == "copy" && _request.DirectClipboardCopy)
        {
            try
            {
                var clipboardStarted = Environment.TickCount64;
                WriteImageToClipboardWithRetry(png);
                clipboardMs = Environment.TickCount64 - clipboardStarted;
                clipboardWritten = true;
            }
            catch (Exception ex)
            {
                ShowFeedback("复制失败: " + ex.Message);
                return;
            }
        }

        var pinX = _capture.Bounds.X + physical.X;
        var pinY = _capture.Bounds.Y + physical.Y;

        var pinned = false;
        var nativePinId = "";
        if (action == "pin" || (action != "qr" && _request.AutoPin))
        {
            try
            {
                nativePinId = Guid.NewGuid().ToString("N");
                new PinWindow(png, pinX, pinY, physical.Width, physical.Height, nativePinId, _request.CallbackPipeName).Show();
                pinned = true;
            }
            catch (Exception ex)
            {
                Complete(new CaptureResponse { Ok = false, Message = "贴图失败: " + ex.Message });
                return;
            }
        }

        Complete(new CaptureResponse
        {
            Ok = true,
            Action = action,
            Message = "",
            PngBase64 = Convert.ToBase64String(png),
            X = pinX,
            Y = pinY,
            Width = physical.Width,
            Height = physical.Height,
            SavedPath = savedPath,
            ClipboardWritten = clipboardWritten,
            Pinned = pinned,
            PinPositioned = pinned,
            PinX = pinX,
            PinY = pinY,
            NativePinId = nativePinId,
            RenderMs = renderMs,
            ClipboardMs = clipboardMs,
            TotalMs = Environment.TickCount64 - totalStarted,
            Operations = ExportOperations(selection, physical)
        });
    }

    private async Task FinishRedactCopyAsync()
    {
        var totalStarted = Environment.TickCount64;
        if (_ocrBusy || _redactBusy)
        {
            ShowPersistentFeedback(_redactBusy ? "正在打码并复制" : "OCR 识别中");
            return;
        }

        CommitTextAnnotation();
        var physical = SelectionPhysical();
        var selection = SelectionDip();
        if (physical.Width < 2 || physical.Height < 2 || selection.Width < 2 || selection.Height < 2)
        {
            ShowFeedback("先拖拽选择区域");
            return;
        }

        byte[] png;
        long renderMs;
        try
        {
            var renderStarted = Environment.TickCount64;
            png = RenderSelectionPng(physical, selection);
            renderMs = Environment.TickCount64 - renderStarted;
        }
        catch (Exception ex)
        {
            ShowFeedback("截图失败: " + ex.Message);
            return;
        }

        var tempPath = System.IO.Path.Combine(System.IO.Path.GetTempPath(), "ariadne-redact-copy-" + Guid.NewGuid().ToString("N") + ".png");
        SetRedactBusy(true);
        try
        {
            ShowPersistentFeedback("正在打码并复制");
            await File.WriteAllBytesAsync(tempPath, png);
            var redactStarted = Environment.TickCount64;
            var response = await PinActionClient.SendAsync(_request.CallbackPipeName, new PinActionRequest
            {
                Action = "redact_copy",
                NativePinId = Guid.NewGuid().ToString("N"),
                ImagePath = tempPath
            });
            var clipboardMs = Environment.TickCount64 - redactStarted;
            if (!response.Ok)
            {
                ShowFeedback(string.IsNullOrWhiteSpace(response.Message) ? "打码复制失败" : response.Message);
                return;
            }
            if (!response.ClipboardWritten)
            {
                ShowFeedback("打码复制失败: 剪贴板未写入");
                return;
            }
            if (string.IsNullOrWhiteSpace(response.PngBase64))
            {
                ShowFeedback("打码复制失败: 截图数据无效");
                return;
            }

            byte[] redactedPng;
            try
            {
                redactedPng = Convert.FromBase64String(response.PngBase64);
            }
            catch (Exception ex)
            {
                ShowFeedback("打码复制失败: " + ex.Message);
                return;
            }

            var pinX = _capture.Bounds.X + physical.X;
            var pinY = _capture.Bounds.Y + physical.Y;
            var pinned = false;
            var nativePinId = "";
            if (_request.AutoPin)
            {
                try
                {
                    nativePinId = Guid.NewGuid().ToString("N");
                    new PinWindow(redactedPng, pinX, pinY, physical.Width, physical.Height, nativePinId, _request.CallbackPipeName).Show();
                    pinned = true;
                }
                catch (Exception ex)
                {
                    ShowFeedback("贴图失败: " + ex.Message);
                    return;
                }
            }

            Complete(new CaptureResponse
            {
                Ok = true,
                Action = "redact_copy",
                Message = string.IsNullOrWhiteSpace(response.Message) ? "已打码复制" : response.Message,
                PngBase64 = response.PngBase64,
                X = pinX,
                Y = pinY,
                Width = response.Width > 0 ? response.Width : physical.Width,
                Height = response.Height > 0 ? response.Height : physical.Height,
                ClipboardWritten = true,
                Pinned = pinned,
                PinPositioned = pinned,
                PinX = pinX,
                PinY = pinY,
                NativePinId = nativePinId,
                RenderMs = renderMs,
                ClipboardMs = clipboardMs,
                TotalMs = Environment.TickCount64 - totalStarted,
                Operations = ExportOperations(selection, physical)
            });
        }
        catch (Exception ex)
        {
            ShowFeedback("打码复制失败: " + ex.Message);
        }
        finally
        {
            SetRedactBusy(false);
            TryDeleteFile(tempPath);
        }
    }

    private static void WriteImageToClipboardWithRetry(byte[] png)
    {
        NativeClipboard.WritePngWithRetry(png);
    }

    private async Task RecognizeSelectionOcrAsync()
    {
        if (_ocrBusy || _redactBusy)
        {
            ShowPersistentFeedback(_redactBusy ? "正在打码并复制" : "OCR 识别中");
            return;
        }

        CommitTextAnnotation();
        var physical = SelectionPhysical();
        var selection = SelectionDip();
        if (physical.Width < 2 || physical.Height < 2 || selection.Width < 2 || selection.Height < 2)
        {
            ShowFeedback("先拖拽选择区域");
            return;
        }

        var tempPath = System.IO.Path.Combine(System.IO.Path.GetTempPath(), "ariadne-ocr-selection-" + Guid.NewGuid().ToString("N") + ".png");
        SetOcrBusy(true);
        try
        {
            ShowPersistentFeedback("OCR 识别中");
            await File.WriteAllBytesAsync(tempPath, _capture.CropPng(physical));
            var response = await PinActionClient.SendAsync(_request.CallbackPipeName, new PinActionRequest
            {
                Action = "ocr_text",
                NativePinId = Guid.NewGuid().ToString("N"),
                ImagePath = tempPath
            });

            if (!response.Ok)
            {
                ShowFeedback(string.IsNullOrWhiteSpace(response.Message) ? "OCR 识别失败" : response.Message);
                return;
            }

            var text = response.Text.Trim();
            if (text.Length == 0)
            {
                ShowFeedback("未识别到文字");
                return;
            }

            Clipboard.SetText(text);
            Complete(new CaptureResponse { Ok = true, Action = "ocr", Message = "OCR 文本已复制" });
        }
        catch (Exception ex)
        {
            ShowFeedback("OCR 识别失败: " + ex.Message);
        }
        finally
        {
            SetOcrBusy(false);
            TryDeleteFile(tempPath);
        }
    }

    private static void TryDeleteFile(string path)
    {
        try
        {
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
        catch
        {
        }
    }

    private byte[] RenderSelectionPng(Int32Rect physical, Rect selection)
    {
        var crop = _capture.CropSource(physical);
        if (_operations.Count == 0)
        {
            return _capture.CropPng(physical);
        }

        var visual = new DrawingVisual();
        using (var context = visual.RenderOpen())
        {
            var scaleX = physical.Width / Math.Max(1, selection.Width);
            var scaleY = physical.Height / Math.Max(1, selection.Height);
            context.PushTransform(new ScaleTransform(scaleX, scaleY));
            context.DrawImage(crop, new Rect(0, 0, selection.Width, selection.Height));
            foreach (var operation in _operations)
            {
                DrawOperation(context, operation, crop, selection);
            }
            context.Pop();
        }

        var bitmap = new RenderTargetBitmap(physical.Width, physical.Height, 96, 96, PixelFormats.Pbgra32);
        bitmap.Render(visual);
        bitmap.Freeze();
        var encoder = new PngBitmapEncoder();
        encoder.Frames.Add(BitmapFrame.Create(bitmap));
        using var stream = new MemoryStream();
        encoder.Save(stream);
        return stream.ToArray();
    }

    private void DrawOperation(DrawingContext context, AnnotationOperation operation, BitmapSource crop, Rect selection)
    {
        var kind = NormalizeKind(operation.Kind);
        var color = ColorFromHex(string.IsNullOrWhiteSpace(operation.Color) ? _annotationColor : operation.Color);
        var brush = new SolidColorBrush(color);
        var stroke = Math.Max(1, operation.StrokeWidth == 0 ? _thickness : operation.StrokeWidth);
        var pen = new Pen(brush, stroke) { StartLineCap = PenLineCap.Round, EndLineCap = PenLineCap.Round, LineJoin = PenLineJoin.Round };
        switch (kind)
        {
            case "rect":
                context.DrawRectangle(null, pen, NormalizeRect(operation));
                break;
            case "line":
                context.DrawLine(pen, new WpfPoint(operation.X, operation.Y), new WpfPoint(operation.EndX, operation.EndY));
                break;
            case "arrow":
                context.DrawLine(pen, new WpfPoint(operation.X, operation.Y), new WpfPoint(operation.EndX, operation.EndY));
                foreach (var point in ArrowHead(operation.X, operation.Y, operation.EndX, operation.EndY, stroke))
                {
                    context.DrawLine(pen, new WpfPoint(operation.EndX, operation.EndY), point);
                }
                break;
            case "pen":
                DrawPath(context, pen, operation.Points ?? []);
                break;
            case "highlight":
                var highlightColor = color;
                highlightColor.A = 108;
                DrawPath(context, new Pen(new SolidColorBrush(highlightColor), Math.Max(10, stroke * 4)) { StartLineCap = PenLineCap.Round, EndLineCap = PenLineCap.Round, LineJoin = PenLineJoin.Round }, operation.Points ?? []);
                break;
            case "mosaic":
                DrawMosaic(context, operation, crop, selection);
                break;
            case "eraser":
                DrawEraser(context, operation, crop, selection);
                break;
            case "text":
                DrawText(context, operation, brush);
                break;
            case "number":
                DrawNumber(context, operation, brush, pen);
                break;
        }
    }

    private static void DrawPath(DrawingContext context, Pen pen, IReadOnlyList<AnnotationPoint> points)
    {
        if (points.Count == 0)
        {
            return;
        }
        if (points.Count == 1)
        {
            context.DrawEllipse(pen.Brush, null, new WpfPoint(points[0].X, points[0].Y), pen.Thickness / 2, pen.Thickness / 2);
            return;
        }
        var geometry = new StreamGeometry();
        using (var stream = geometry.Open())
        {
            stream.BeginFigure(new WpfPoint(points[0].X, points[0].Y), false, false);
            for (var index = 1; index < points.Count; index++)
            {
                stream.LineTo(new WpfPoint(points[index].X, points[index].Y), true, false);
            }
        }
        geometry.Freeze();
        context.DrawGeometry(null, pen, geometry);
    }

    private void DrawMosaic(DrawingContext context, AnnotationOperation operation, BitmapSource crop, Rect selection)
    {
        var block = Math.Max(6, operation.PixelSize == 0 ? _thickness * 4 : operation.PixelSize);
        if (operation.Points is { Count: > 1 } points)
        {
            foreach (var point in points)
            {
                var rect = new Rect(point.X - block, point.Y - block, block * 2, block * 2);
                DrawMosaicBlocks(context, rect, block, crop, selection);
            }
            return;
        }
        DrawMosaicBlocks(context, NormalizeRect(operation), block, crop, selection);
    }

    private static void DrawMosaicBlocks(DrawingContext context, Rect rect, int block, BitmapSource crop, Rect selection)
    {
        if (rect.Width <= 0 || rect.Height <= 0)
        {
            return;
        }
        BitmapSource sampleSource = crop;
        if (sampleSource.Format != PixelFormats.Bgra32)
        {
            var converted = new FormatConvertedBitmap(sampleSource, PixelFormats.Bgra32, null, 0);
            converted.Freeze();
            sampleSource = converted;
        }
        var scaleX = sampleSource.PixelWidth / Math.Max(1, selection.Width);
        var scaleY = sampleSource.PixelHeight / Math.Max(1, selection.Height);
        var clipped = rect;
        clipped.Intersect(new Rect(0, 0, selection.Width, selection.Height));
        for (var y = rect.Y; y < rect.Bottom; y += block)
        {
            for (var x = rect.X; x < rect.Right; x += block)
            {
                var drawRect = new Rect(x, y, Math.Min(block, rect.Right - x), Math.Min(block, rect.Bottom - y));
                drawRect.Intersect(clipped);
                if (drawRect.Width <= 0 || drawRect.Height <= 0)
                {
                    continue;
                }
                var sampleX = (int)Math.Round((drawRect.X + drawRect.Width / 2) * scaleX);
                var sampleY = (int)Math.Round((drawRect.Y + drawRect.Height / 2) * scaleY);
                context.DrawRectangle(new SolidColorBrush(SampleBitmapPixel(sampleSource, sampleX, sampleY)), null, drawRect);
            }
        }
    }

    private static WpfColor SampleBitmapPixel(BitmapSource source, int x, int y)
    {
        x = Math.Clamp(x, 0, source.PixelWidth - 1);
        y = Math.Clamp(y, 0, source.PixelHeight - 1);
        var buffer = new byte[4];
        source.CopyPixels(new Int32Rect(x, y, 1, 1), buffer, 4, 0);
        return WpfColor.FromRgb(buffer[2], buffer[1], buffer[0]);
    }

    private static void DrawEraser(DrawingContext context, AnnotationOperation operation, BitmapSource crop, Rect selection)
    {
        var points = operation.Points ?? [];
        if (points.Count == 0)
        {
            return;
        }
        var radius = Math.Max(4, operation.StrokeWidth * 2);
        foreach (var point in PathSamples(points, Math.Max(2, radius / 2)))
        {
            context.PushClip(new EllipseGeometry(new WpfPoint(point.X, point.Y), radius, radius));
            context.DrawImage(crop, new Rect(0, 0, selection.Width, selection.Height));
            context.Pop();
        }
    }

    private void DrawText(DrawingContext context, AnnotationOperation operation, Brush brush)
    {
        var fontSize = Math.Max(12, operation.FontSize == 0 ? 20 : operation.FontSize);
        var pixelsPerDip = VisualTreeHelper.GetDpi(this).PixelsPerDip;
        var text = new FormattedText(
            operation.Text ?? "",
            CultureInfo.CurrentUICulture,
            FlowDirection.LeftToRight,
            new Typeface(new FontFamily("Microsoft YaHei UI, Segoe UI"), FontStyles.Normal, FontWeights.SemiBold, FontStretches.Normal),
            fontSize,
            brush,
            pixelsPerDip)
        {
            MaxTextWidth = Math.Max(48, operation.Width == 0 ? 220 : operation.Width)
        };
        context.DrawText(text, new WpfPoint(operation.X, operation.Y));
    }

    private void DrawNumber(DrawingContext context, AnnotationOperation operation, Brush brush, Pen pen)
    {
        var radius = Math.Max(10, operation.FontSize == 0 ? 18 : operation.FontSize / 2.0);
        var center = new WpfPoint(operation.X, operation.Y);
        context.DrawEllipse(NativeVisuals.Brush(238, 255, 255, 255), pen, center, radius, radius);
        var pixelsPerDip = VisualTreeHelper.GetDpi(this).PixelsPerDip;
        var text = new FormattedText(
            Math.Max(1, operation.Number).ToString(CultureInfo.InvariantCulture),
            CultureInfo.CurrentUICulture,
            FlowDirection.LeftToRight,
            new Typeface(new FontFamily("Segoe UI"), FontStyles.Normal, FontWeights.Bold, FontStretches.Normal),
            Math.Max(12, radius),
            brush,
            pixelsPerDip);
        context.DrawText(text, new WpfPoint(center.X - text.Width / 2, center.Y - text.Height / 2));
    }

    private List<AnnotationOperation> ExportOperations(Rect selection, Int32Rect physical)
    {
        var scaleX = physical.Width / Math.Max(1, selection.Width);
        var scaleY = physical.Height / Math.Max(1, selection.Height);
        return _operations.Select(operation => ScaleOperation(operation, scaleX, scaleY)).ToList();
    }

    private AnnotationOperation CreateAnnotation(WpfPoint start, WpfPoint end)
    {
        var kind = ToolKind(_tool);
        var operation = new AnnotationOperation
        {
            Kind = kind,
            X = (int)Math.Round(start.X),
            Y = (int)Math.Round(start.Y),
            Width = (int)Math.Round(end.X - start.X),
            Height = (int)Math.Round(end.Y - start.Y),
            EndX = (int)Math.Round(end.X),
            EndY = (int)Math.Round(end.Y),
            Color = _annotationColor,
            StrokeWidth = _thickness,
            PixelSize = Math.Max(6, _thickness * 4)
        };
        if (IsPathTool(_tool))
        {
            operation.Points =
            [
                new AnnotationPoint { X = operation.X, Y = operation.Y },
                new AnnotationPoint { X = operation.EndX, Y = operation.EndY }
            ];
        }
        return operation;
    }

    private static void AppendPoint(AnnotationOperation operation, WpfPoint point)
    {
        operation.Points ??= [];
        operation.Points.Add(new AnnotationPoint { X = (int)Math.Round(point.X), Y = (int)Math.Round(point.Y) });
        operation.EndX = (int)Math.Round(point.X);
        operation.EndY = (int)Math.Round(point.Y);
    }

    private static AnnotationOperation CloneOperation(AnnotationOperation operation)
    {
        return new AnnotationOperation
        {
            Kind = operation.Kind,
            X = operation.X,
            Y = operation.Y,
            Width = operation.Width,
            Height = operation.Height,
            EndX = operation.EndX,
            EndY = operation.EndY,
            Color = operation.Color,
            StrokeWidth = operation.StrokeWidth,
            PixelSize = operation.PixelSize,
            Points = operation.Points?.Select(point => new AnnotationPoint { X = point.X, Y = point.Y }).ToList(),
            Text = operation.Text,
            FontSize = operation.FontSize,
            Number = operation.Number
        };
    }

    private bool ApplyEraser(AnnotationOperation eraser)
    {
        var eraserSamples = PathSamples(eraser.Points ?? [], Math.Max(2, eraser.StrokeWidth)).ToList();
        if (eraserSamples.Count == 0)
        {
            return false;
        }

        var changed = false;
        var nextOperations = new List<AnnotationOperation>();
        foreach (var operation in _operations)
        {
            var erased = EraseOperation(operation, eraser, eraserSamples);
            if (erased.Count != 1 || !ReferenceEquals(erased[0], operation))
            {
                changed = true;
            }
            nextOperations.AddRange(erased);
        }

        if (!changed)
        {
            return false;
        }

        _operations.Clear();
        _operations.AddRange(nextOperations);
        return true;
    }

    private static List<AnnotationOperation> EraseOperation(AnnotationOperation operation, AnnotationOperation eraser, IReadOnlyList<AnnotationPoint> eraserSamples)
    {
        var kind = NormalizeKind(operation.Kind);
        var radius = Math.Max(4, eraser.StrokeWidth * 2);
        if ((kind is "pen" or "highlight" or "mosaic") && operation.Points is { Count: > 1 } points)
        {
            return ErasePathOperation(operation, points, eraserSamples, radius + Math.Max(1, operation.StrokeWidth / 2.0));
        }

        return OperationHitByEraser(operation, eraserSamples, radius)
            ? []
            : [operation];
    }

    private static List<AnnotationOperation> ErasePathOperation(
        AnnotationOperation operation,
        IReadOnlyList<AnnotationPoint> points,
        IReadOnlyList<AnnotationPoint> eraserSamples,
        double radius)
    {
        var segments = new List<AnnotationOperation>();
        var segment = new List<AnnotationPoint>();
        foreach (var point in points)
        {
            if (EraserHitsPoint(eraserSamples, point, radius))
            {
                AddErasedSegment(segments, operation, segment);
                segment.Clear();
                continue;
            }
            segment.Add(new AnnotationPoint { X = point.X, Y = point.Y });
        }
        AddErasedSegment(segments, operation, segment);
        return segments;
    }

    private static void AddErasedSegment(List<AnnotationOperation> segments, AnnotationOperation source, IReadOnlyList<AnnotationPoint> points)
    {
        if (points.Count < 2)
        {
            return;
        }

        var first = points[0];
        var last = points[^1];
        var next = CloneOperation(source);
        next.Points = points.Select(point => new AnnotationPoint { X = point.X, Y = point.Y }).ToList();
        next.X = first.X;
        next.Y = first.Y;
        next.EndX = last.X;
        next.EndY = last.Y;
        next.Width = last.X - first.X;
        next.Height = last.Y - first.Y;
        segments.Add(next);
    }

    private static bool OperationHitByEraser(AnnotationOperation operation, IReadOnlyList<AnnotationPoint> eraserSamples, double radius)
    {
        var bounds = OperationBounds(operation);
        if (bounds.IsEmpty)
        {
            return false;
        }
        bounds.Inflate(radius, radius);
        return eraserSamples.Any(point => bounds.Contains(new WpfPoint(point.X, point.Y)));
    }

    private static bool EraserHitsPoint(IReadOnlyList<AnnotationPoint> eraserSamples, AnnotationPoint point, double radius)
    {
        var radiusSquared = radius * radius;
        return eraserSamples.Any(sample =>
        {
            var dx = sample.X - point.X;
            var dy = sample.Y - point.Y;
            return dx * dx + dy * dy <= radiusSquared;
        });
    }

    private static AnnotationOperation TranslateOperation(AnnotationOperation operation, int dx, int dy)
    {
        var next = CloneOperation(operation);
        next.X += dx;
        next.Y += dy;
        next.EndX += dx;
        next.EndY += dy;
        if (next.Points != null)
        {
            foreach (var point in next.Points)
            {
                point.X += dx;
                point.Y += dy;
            }
        }
        return next;
    }

    private static AnnotationOperation ScaleOperation(AnnotationOperation operation, double scaleX, double scaleY)
    {
        var scaleStroke = Math.Max(1, (int)Math.Round(operation.StrokeWidth * Math.Max(scaleX, scaleY)));
        return new AnnotationOperation
        {
            Kind = operation.Kind,
            X = (int)Math.Round(operation.X * scaleX),
            Y = (int)Math.Round(operation.Y * scaleY),
            Width = (int)Math.Round(operation.Width * scaleX),
            Height = (int)Math.Round(operation.Height * scaleY),
            EndX = (int)Math.Round(operation.EndX * scaleX),
            EndY = (int)Math.Round(operation.EndY * scaleY),
            Color = operation.Color,
            StrokeWidth = scaleStroke,
            PixelSize = Math.Max(1, (int)Math.Round(operation.PixelSize * Math.Max(scaleX, scaleY))),
            Points = operation.Points?.Select(point => new AnnotationPoint
            {
                X = (int)Math.Round(point.X * scaleX),
                Y = (int)Math.Round(point.Y * scaleY)
            }).ToList(),
            Text = operation.Text,
            FontSize = Math.Max(1, (int)Math.Round(operation.FontSize * Math.Max(scaleX, scaleY))),
            Number = operation.Number
        };
    }

    private static bool IsUsefulAnnotation(AnnotationOperation operation)
    {
        return NormalizeKind(operation.Kind) switch
        {
            "rect" or "mosaic" => Math.Abs(operation.Width) >= 4 && Math.Abs(operation.Height) >= 4 || operation.Points is { Count: > 2 },
            "line" or "arrow" => Math.Abs(operation.EndX - operation.X) + Math.Abs(operation.EndY - operation.Y) >= 6,
            "pen" or "highlight" or "eraser" => operation.Points is { Count: > 2 },
            "text" => !string.IsNullOrWhiteSpace(operation.Text),
            "number" => true,
            _ => false
        };
    }

    private static bool IsPathTool(AnnotationTool tool)
    {
        return tool is AnnotationTool.Pen or AnnotationTool.Highlight or AnnotationTool.Mosaic or AnnotationTool.Eraser;
    }

    private static string ToolKind(AnnotationTool tool)
    {
        return tool switch
        {
            AnnotationTool.Rect => "rect",
            AnnotationTool.Line => "line",
            AnnotationTool.Arrow => "arrow",
            AnnotationTool.Pen => "pen",
            AnnotationTool.Highlight => "highlight",
            AnnotationTool.Mosaic => "mosaic",
            AnnotationTool.Text => "text",
            AnnotationTool.Number => "number",
            AnnotationTool.Eraser => "eraser",
            _ => ""
        };
    }

    private static string ToolLabel(AnnotationTool tool)
    {
        return tool switch
        {
            AnnotationTool.Rect => "矩形",
            AnnotationTool.Line => "直线",
            AnnotationTool.Arrow => "箭头",
            AnnotationTool.Pen => "画笔",
            AnnotationTool.Highlight => "荧光笔",
            AnnotationTool.Mosaic => "马赛克",
            AnnotationTool.Text => "文字",
            AnnotationTool.Number => "序号",
            AnnotationTool.Eraser => "橡皮擦",
            AnnotationTool.Select => "选择并移动标注",
            _ => ""
        };
    }

    private static string NormalizeKind(string kind)
    {
        return kind.Trim().ToLowerInvariant() switch
        {
            "rectangle" => "rect",
            "brush" => "pen",
            "highlighter" => "highlight",
            "marker" => "highlight",
            "pixelate" => "mosaic",
            var value => value
        };
    }

    private WpfPoint ToSelectionPoint(WpfPoint point)
    {
        var selection = SelectionDip();
        return new WpfPoint(
            Math.Clamp(point.X - selection.X, 0, Math.Max(0, selection.Width)),
            Math.Clamp(point.Y - selection.Y, 0, Math.Max(0, selection.Height)));
    }

    private static Rect NormalizeRect(AnnotationOperation operation)
    {
        var x1 = Math.Min(operation.X, operation.X + operation.Width);
        var x2 = Math.Max(operation.X, operation.X + operation.Width);
        var y1 = Math.Min(operation.Y, operation.Y + operation.Height);
        var y2 = Math.Max(operation.Y, operation.Y + operation.Height);
        return new Rect(x1, y1, Math.Max(0, x2 - x1), Math.Max(0, y2 - y1));
    }

    private static Rect OperationBounds(AnnotationOperation operation)
    {
        var kind = NormalizeKind(operation.Kind);
        if (operation.Points is { Count: > 0 } points)
        {
            var left = points.Min(point => point.X);
            var top = points.Min(point => point.Y);
            var right = points.Max(point => point.X);
            var bottom = points.Max(point => point.Y);
            var pad = Math.Max(8, operation.StrokeWidth * 2);
            return new Rect(left - pad, top - pad, Math.Max(1, right - left + pad * 2), Math.Max(1, bottom - top + pad * 2));
        }
        if (kind is "rect" or "mosaic")
        {
            return NormalizeRect(operation);
        }
        if (kind is "line" or "arrow")
        {
            var left = Math.Min(operation.X, operation.EndX);
            var top = Math.Min(operation.Y, operation.EndY);
            var right = Math.Max(operation.X, operation.EndX);
            var bottom = Math.Max(operation.Y, operation.EndY);
            return new Rect(left, top, Math.Max(1, right - left), Math.Max(1, bottom - top));
        }
        if (kind == "text")
        {
            return new Rect(operation.X, operation.Y, Math.Max(48, operation.Width), Math.Max(24, operation.FontSize * 2));
        }
        if (kind == "number")
        {
            var radius = Math.Max(12, operation.FontSize / 2);
            return new Rect(operation.X - radius, operation.Y - radius, radius * 2, radius * 2);
        }
        return Rect.Empty;
    }

    private static IEnumerable<WpfPoint> ArrowHead(double x1, double y1, double x2, double y2, double stroke)
    {
        var angle = Math.Atan2(y2 - y1, x2 - x1);
        var headLength = Math.Max(12, stroke * 4);
        yield return new WpfPoint(x2 - Math.Cos(angle - Math.PI / 6) * headLength, y2 - Math.Sin(angle - Math.PI / 6) * headLength);
        yield return new WpfPoint(x2 - Math.Cos(angle + Math.PI / 6) * headLength, y2 - Math.Sin(angle + Math.PI / 6) * headLength);
    }

    private static IEnumerable<AnnotationPoint> PathSamples(IReadOnlyList<AnnotationPoint> points, int step)
    {
        if (points.Count == 0)
        {
            yield break;
        }
        yield return points[0];
        for (var index = 1; index < points.Count; index++)
        {
            var start = points[index - 1];
            var end = points[index];
            var dx = end.X - start.X;
            var dy = end.Y - start.Y;
            var distance = Math.Max(Math.Abs(dx), Math.Abs(dy));
            var samples = Math.Max(1, (int)Math.Ceiling(distance / (double)Math.Max(1, step)));
            for (var sample = 1; sample <= samples; sample++)
            {
                yield return new AnnotationPoint
                {
                    X = start.X + (int)Math.Round(dx * sample / (double)samples),
                    Y = start.Y + (int)Math.Round(dy * sample / (double)samples)
                };
            }
        }
    }

    private void UpdateMagnifier(WpfPoint point)
    {
        if (ActualWidth <= 0 || ActualHeight <= 0)
        {
            return;
        }
        var physicalX = LocalToPhysicalX(point.X);
        var physicalY = LocalToPhysicalY(point.Y);
        var now = Environment.TickCount64;
        var refreshContent = _magnifier.Visibility != Visibility.Visible
            || now - _lastMagnifierContentMs >= MagnifierContentFrameMs
            || Math.Abs(physicalX - _lastMagnifierPhysicalX) >= 8
            || Math.Abs(physicalY - _lastMagnifierPhysicalY) >= 8;
        if (refreshContent)
        {
            var sampleSize = 24;
            var left = Math.Clamp(physicalX - sampleSize / 2, 0, Math.Max(0, _capture.Source.PixelWidth - sampleSize));
            var top = Math.Clamp(physicalY - sampleSize / 2, 0, Math.Max(0, _capture.Source.PixelHeight - sampleSize));
            _magnifierImage.Source = _capture.CropSource(new Int32Rect(left, top, Math.Min(sampleSize, _capture.Source.PixelWidth - left), Math.Min(sampleSize, _capture.Source.PixelHeight - top)));
            var color = _capture.SamplePixel(physicalX, physicalY);
            _magnifierText.Text = FormatColor(color, ColorFormatPreferences.Current);
            _lastMagnifierContentMs = now;
            _lastMagnifierPhysicalX = physicalX;
            _lastMagnifierPhysicalY = physicalY;
        }
        _magnifier.Visibility = Visibility.Visible;

        var x = point.X + 24;
        var y = point.Y + 24;
        if (x + _magnifier.Width > ActualWidth - 12)
        {
            x = point.X - _magnifier.Width - 24;
        }
        if (y + 148 > ActualHeight - 12)
        {
            y = point.Y - 148;
        }
        SetElement(_magnifier, Math.Clamp(x, 12, Math.Max(12, ActualWidth - _magnifier.Width - 12)), Math.Clamp(y, 12, Math.Max(12, ActualHeight - 148)), _magnifier.Width, 148);
    }

    private void SetSelectionDragVisuals(bool active)
    {
        if (_selectionDragVisualsActive == active)
        {
            return;
        }

        _selectionDragVisualsActive = active;
        _selection.Effect = active ? null : _selectionIdleEffect;
    }

    private void CopyPointerColor()
    {
        var color = _capture.SamplePixel(LocalToPhysicalX(_lastPoint.X), LocalToPhysicalY(_lastPoint.Y));
        var text = FormatColor(color, ColorFormatPreferences.Current);
        try
        {
            NativeClipboard.WriteTextWithRetry(text);
            Complete(new CaptureResponse
            {
                Ok = true,
                Action = "color",
                Message = "颜色已复制: " + text,
                ClipboardWritten = true
            });
        }
        catch (Exception ex)
        {
            ShowFeedback("复制失败: " + ex.Message);
        }
    }

    private void RefreshPointerColorText()
    {
        if (_magnifier.Visibility != Visibility.Visible)
        {
            return;
        }
        var color = _capture.SamplePixel(LocalToPhysicalX(_lastPoint.X), LocalToPhysicalY(_lastPoint.Y));
        _magnifierText.Text = FormatColor(color, ColorFormatPreferences.Current);
    }

    private void SetThickness(int value, bool showFeedback)
    {
        var next = Math.Clamp(value, 1, 24);
        _thickness = next;
        if (_thicknessSlider != null && (int)Math.Round(_thicknessSlider.Value) != next)
        {
            _thicknessSlider.Value = next;
        }
        if (_thicknessText != null)
        {
            _thicknessText.Text = next.ToString(CultureInfo.InvariantCulture);
        }
        if (showFeedback)
        {
            ShowFeedback("粗细 " + next.ToString(CultureInfo.InvariantCulture));
        }
    }

    private static string FormatColor(WpfColor color, ColorFormat format)
    {
        return format == ColorFormat.Hex
            ? $"#{color.R:X2}{color.G:X2}{color.B:X2}"
            : $"rgb({color.R}, {color.G}, {color.B})";
    }

    private void ShowFeedback(string message)
    {
        _feedbackText.Text = message;
        _feedback.Visibility = Visibility.Visible;
        PositionOverlayChrome();
        _feedbackTimer.Stop();
        _feedbackTimer.Start();
    }

    private void ShowPersistentFeedback(string message)
    {
        _feedbackText.Text = message;
        _feedback.Visibility = Visibility.Visible;
        PositionOverlayChrome();
        _feedbackTimer.Stop();
    }

    private void SetOcrBusy(bool busy)
    {
        _ocrBusy = busy;
        UpdateToolStates();
    }

    private void SetRedactBusy(bool busy)
    {
        _redactBusy = busy;
        UpdateToolStates();
    }

    private void ApplyOcrButtonState()
    {
        if (_ocrButton == null)
        {
            return;
        }

        if (_ocrBusy || _redactBusy)
        {
            _ocrButton.IsEnabled = false;
            _ocrButton.Background = NativeVisuals.Brush(235, 31, 41, 51);
            _ocrButton.Foreground = NativeVisuals.Brush(255, 255, 255);
            _ocrButton.ToolTip = _redactBusy ? "正在打码并复制" : "OCR 识别中";
            _ocrButton.Opacity = 1;
            return;
        }

        var hasSelection = SelectionDip().Width >= 2 && SelectionDip().Height >= 2;
        _ocrButton.IsEnabled = hasSelection;
        _ocrButton.Background = WpfBrushes.Transparent;
        _ocrButton.Foreground = NativeVisuals.Brush(39, 39, 42);
        _ocrButton.ToolTip = "OCR 并复制文字";
        _ocrButton.Opacity = 1;
    }

    private void UpdateToolStates()
    {
        var hasSelection = SelectionDip().Width >= 2 && SelectionDip().Height >= 2;
        var busy = _ocrBusy || _redactBusy;
        foreach (var button in _selectionButtons)
        {
            button.IsEnabled = hasSelection && !busy;
        }
        foreach (var (button, tool) in _toolButtons)
        {
            var active = tool == _tool && (tool == AnnotationTool.Select || _editMode);
            button.IsEnabled = hasSelection && !busy;
            button.Background = active ? NativeVisuals.Brush(48, 31, 41, 51) : WpfBrushes.Transparent;
            button.Foreground = active ? NativeVisuals.Brush(31, 41, 51) : NativeVisuals.Brush(39, 39, 42);
        }
        foreach (var button in _operationButtons)
        {
            button.IsEnabled = hasSelection && !busy && (button.ToolTip?.ToString()?.Contains("重做", StringComparison.Ordinal) == true ? _redoOperations.Count > 0 : _operations.Count > 0);
        }
        foreach (var button in _colorButtons)
        {
            button.IsEnabled = !busy;
            button.BorderThickness = string.Equals(button.ToolTip?.ToString(), _annotationColor, StringComparison.OrdinalIgnoreCase) ? new Thickness(2) : new Thickness(1);
            button.BorderBrush = string.Equals(button.ToolTip?.ToString(), _annotationColor, StringComparison.OrdinalIgnoreCase) ? NativeVisuals.Brush(31, 41, 51) : NativeVisuals.Brush(190, 212, 212, 216);
        }
        ApplyOcrButtonState();
    }

    private WpfPoint ClampPoint(WpfPoint point)
    {
        return new WpfPoint(
            Math.Clamp(point.X, 0, Math.Max(ActualWidth, Width)),
            Math.Clamp(point.Y, 0, Math.Max(ActualHeight, Height)));
    }

    private Rect SelectionDip()
    {
        var x1 = Math.Clamp(Math.Min(_start.X, _end.X), 0, Math.Max(ActualWidth, Width));
        var y1 = Math.Clamp(Math.Min(_start.Y, _end.Y), 0, Math.Max(ActualHeight, Height));
        var x2 = Math.Clamp(Math.Max(_start.X, _end.X), 0, Math.Max(ActualWidth, Width));
        var y2 = Math.Clamp(Math.Max(_start.Y, _end.Y), 0, Math.Max(ActualHeight, Height));
        return new Rect(x1, y1, Math.Max(0, x2 - x1), Math.Max(0, y2 - y1));
    }

    private Int32Rect SelectionPhysical()
    {
        var selection = SelectionDip();
        var x = (int)Math.Floor(selection.X * LocalToPhysicalScaleX());
        var y = (int)Math.Floor(selection.Y * LocalToPhysicalScaleY());
        var right = (int)Math.Ceiling((selection.X + selection.Width) * LocalToPhysicalScaleX());
        var bottom = (int)Math.Ceiling((selection.Y + selection.Height) * LocalToPhysicalScaleY());
        return new Int32Rect(x, y, Math.Max(0, right - x), Math.Max(0, bottom - y));
    }

    private int LocalToPhysicalX(double value)
    {
        return (int)Math.Round(value * LocalToPhysicalScaleX());
    }

    private int LocalToPhysicalY(double value)
    {
        return (int)Math.Round(value * LocalToPhysicalScaleY());
    }

    private double LocalToPhysicalScaleX()
    {
        return _capture.Bounds.Width / Math.Max(1.0, Math.Max(ActualWidth, Width));
    }

    private double LocalToPhysicalScaleY()
    {
        return _capture.Bounds.Height / Math.Max(1.0, Math.Max(ActualHeight, Height));
    }

    private void Complete(CaptureResponse response)
    {
        if (_completed)
        {
            return;
        }
        _completed = true;
        Completed?.Invoke(response);
    }

    private static WpfRectangle Mask()
    {
        return new WpfRectangle
        {
            Fill = new SolidColorBrush(WpfColor.FromArgb(118, 0, 0, 0)),
            IsHitTestVisible = false
        };
    }

    private static void SetRect(WpfRectangle rect, double x, double y, double width, double height)
    {
        SetElement(rect, x, y, width, height);
    }

    private static void SetElement(FrameworkElement element, double x, double y, double width, double height)
    {
        var nextWidth = Math.Max(0, width);
        var nextHeight = Math.Max(0, height);
        if (!AlmostEqual(Canvas.GetLeft(element), x))
        {
            Canvas.SetLeft(element, x);
        }
        if (!AlmostEqual(Canvas.GetTop(element), y))
        {
            Canvas.SetTop(element, y);
        }
        if (!AlmostEqual(element.Width, nextWidth))
        {
            element.Width = nextWidth;
        }
        if (!AlmostEqual(element.Height, nextHeight))
        {
            element.Height = nextHeight;
        }
    }

    private static bool AlmostEqual(double current, double next)
    {
        return !double.IsNaN(current) && Math.Abs(current - next) < 0.1;
    }

    private bool IsControlSource(DependencyObject? source)
    {
        while (source != null)
        {
            if (ReferenceEquals(source, _toolbar) || ReferenceEquals(source, _closeButton))
            {
                return true;
            }
            if (source is ButtonBase or Slider or TextBox)
            {
                return true;
            }
            source = VisualTreeHelper.GetParent(source);
        }
        return false;
    }

    private static WpfColor ColorFromHex(string value)
    {
        try
        {
            return (WpfColor)ColorConverter.ConvertFromString(value)!;
        }
        catch
        {
            return WpfColor.FromRgb(220, 38, 38);
        }
    }
}

internal static class RectExtensions
{
    public static Rect InflateBy(this Rect rect, double value)
    {
        rect.Inflate(value, value);
        return rect;
    }
}
