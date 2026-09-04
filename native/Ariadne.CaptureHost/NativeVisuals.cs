using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Data;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Effects;
using System.Windows.Shapes;

namespace Ariadne.CaptureHost;

internal static class NativeVisuals
{
    public static readonly FontFamily ToolbarFont = new("Microsoft YaHei UI, Segoe UI Symbol, Segoe UI");

    public static SolidColorBrush Brush(byte alpha, byte red, byte green, byte blue)
    {
        var brush = new SolidColorBrush(Color.FromArgb(alpha, red, green, blue));
        brush.Freeze();
        return brush;
    }

    public static SolidColorBrush Brush(byte red, byte green, byte blue)
    {
        var brush = new SolidColorBrush(Color.FromRgb(red, green, blue));
        brush.Freeze();
        return brush;
    }

    public static DropShadowEffect Shadow(byte alpha = 60, double blur = 34, double depth = 12)
    {
        return new DropShadowEffect
        {
            Color = Color.FromArgb(alpha, 15, 23, 42),
            BlurRadius = blur,
            ShadowDepth = depth,
            Direction = 270,
            Opacity = 1
        };
    }

    public static Button IconButton(string glyph, string tooltip, Action action, bool primary = false, double size = 30)
    {
        var button = new Button
        {
            Content = IconContent(glyph),
            Width = size,
            Height = size,
            MinWidth = size,
            Padding = new Thickness(0),
            BorderThickness = new Thickness(0),
            Background = primary ? Brush(235, 31, 41, 51) : Brushes.Transparent,
            Foreground = primary ? Brush(255, 255, 255) : Brush(39, 39, 42),
            ToolTip = tooltip,
            Cursor = Cursors.Arrow,
            Focusable = false,
            Template = RoundButtonTemplate(Math.Max(4, size / 2))
        };
        button.Click += (_, _) => action();
        return button;
    }

    private static UIElement IconContent(string glyph)
    {
        const string prefix = "lucide:";
        if (glyph.StartsWith(prefix, StringComparison.Ordinal))
        {
            return LucideIcon(glyph[prefix.Length..]);
        }
        return new TextBlock
        {
            Text = glyph,
            FontFamily = ToolbarFont,
            FontSize = glyph.Length > 1 ? 11 : 13,
            FontWeight = FontWeights.SemiBold,
            HorizontalAlignment = HorizontalAlignment.Center,
            VerticalAlignment = VerticalAlignment.Center,
            TextAlignment = TextAlignment.Center
        };
    }

    private static Viewbox LucideIcon(string name)
    {
        var canvas = new Canvas
        {
            Width = 24,
            Height = 24,
            IsHitTestVisible = false
        };
        switch (name)
        {
            case "arrow-up-right":
                AddPath(canvas, "M7 17 L17 7 M7 7 H17 V17");
                break;
            case "check":
                AddPath(canvas, "M20 6 L9 17 L4 12");
                break;
            case "circle-slash-2":
                AddEllipse(canvas, 4, 4, 16, 16);
                AddLine(canvas, 8, 16, 16, 8);
                break;
            case "copy":
                AddRectangle(canvas, 9, 9, 10, 10);
                AddPath(canvas, "M5 15 H4 A1 1 0 0 1 3 14 V4 A1 1 0 0 1 4 3 H14 A1 1 0 0 1 15 4 V5");
                break;
            case "eraser":
                AddPath(canvas, "M7 21 H21 M5 14 L13 6 L18 11 L10 19 Z M10 19 L5 14");
                break;
            case "file-text":
                AddPath(canvas, "M14 2 H6 A2 2 0 0 0 4 4 V20 A2 2 0 0 0 6 22 H18 A2 2 0 0 0 20 20 V8 Z M14 2 V8 H20");
                AddLine(canvas, 8, 13, 16, 13);
                AddLine(canvas, 8, 17, 14, 17);
                break;
            case "grid-3x3":
                AddLine(canvas, 8, 3, 8, 21);
                AddLine(canvas, 16, 3, 16, 21);
                AddLine(canvas, 3, 8, 21, 8);
                AddLine(canvas, 3, 16, 21, 16);
                AddRectangle(canvas, 3, 3, 18, 18);
                break;
            case "hash":
                AddLine(canvas, 4, 9, 20, 9);
                AddLine(canvas, 4, 15, 20, 15);
                AddLine(canvas, 10, 3, 8, 21);
                AddLine(canvas, 16, 3, 14, 21);
                break;
            case "highlighter":
                AddPath(canvas, "M4 20 H13 L20 13 L11 4 L4 11 V20 Z M8 16 L15 9");
                break;
            case "minus":
                AddLine(canvas, 5, 12, 19, 12);
                break;
            case "mouse-pointer-2":
                AddPath(canvas, "M4 3 L18 13 L12 14 L9 21 L4 3 Z");
                break;
            case "pencil":
                AddPath(canvas, "M18 2 L22 6 L9 19 L4 20 L5 15 Z M15 5 L19 9");
                break;
            case "pin":
                AddPath(canvas, "M12 17 V22 M5 17 H19 M7 17 L9 4 H15 L17 17");
                break;
            case "qr-code":
                AddRectangle(canvas, 3, 3, 6, 6);
                AddRectangle(canvas, 15, 3, 6, 6);
                AddRectangle(canvas, 3, 15, 6, 6);
                AddPath(canvas, "M15 15 H17 V17 H15 Z M20 15 V17 M17 20 H20 M12 3 V5 M12 8 V10 M12 14 V16 M12 19 V21");
                break;
            case "redo-2":
                AddPath(canvas, "M21 7 V13 H15 M20 13 C17 8 10 7 6 11 C4 13 3 15 3 18");
                break;
            case "rotate-ccw":
                AddPath(canvas, "M3 8 V3 H8 M4 13 C4 18 8 21 12 21 C17 21 21 17 21 12 C21 7 17 3 12 3 C9 3 6 5 4 8");
                break;
            case "save":
                AddPath(canvas, "M5 3 H16 L21 8 V19 A2 2 0 0 1 19 21 H5 A2 2 0 0 1 3 19 V5 A2 2 0 0 1 5 3 Z M7 3 V8 H15 M7 21 V14 H17 V21");
                break;
            case "shield-check":
                AddPath(canvas, "M12 22 C12 22 20 18 20 10 V5 L12 2 L4 5 V10 C4 18 12 22 12 22 Z M9 12 L11 14 L15 10");
                break;
            case "square":
                AddRectangle(canvas, 5, 5, 14, 14);
                break;
            case "trash-2":
                AddPath(canvas, "M3 6 H21 M8 6 V4 H16 V6 M19 6 L18 20 H6 L5 6 M10 11 V17 M14 11 V17");
                break;
            case "type":
                AddPath(canvas, "M4 7 V4 H20 V7 M9 20 H15 M12 4 V20");
                break;
            case "x":
                AddPath(canvas, "M6 6 L18 18 M18 6 L6 18");
                break;
            default:
                AddEllipse(canvas, 6, 6, 12, 12);
                break;
        }
        return new Viewbox
        {
            Width = 16,
            Height = 16,
            Stretch = Stretch.Uniform,
            IsHitTestVisible = false,
            Child = canvas
        };
    }

    private static void AddPath(Canvas canvas, string data)
    {
        var path = new Path
        {
            Data = Geometry.Parse(data),
            StrokeThickness = 2,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round,
            StrokeLineJoin = PenLineJoin.Round,
            Fill = Brushes.Transparent
        };
        BindStroke(path);
        canvas.Children.Add(path);
    }

    private static void AddLine(Canvas canvas, double x1, double y1, double x2, double y2)
    {
        var line = new Line
        {
            X1 = x1,
            Y1 = y1,
            X2 = x2,
            Y2 = y2,
            StrokeThickness = 2,
            StrokeStartLineCap = PenLineCap.Round,
            StrokeEndLineCap = PenLineCap.Round
        };
        BindStroke(line);
        canvas.Children.Add(line);
    }

    private static void AddRectangle(Canvas canvas, double x, double y, double width, double height)
    {
        var rectangle = new Rectangle
        {
            Width = width,
            Height = height,
            StrokeThickness = 2,
            RadiusX = 1.5,
            RadiusY = 1.5,
            Fill = Brushes.Transparent
        };
        BindStroke(rectangle);
        Canvas.SetLeft(rectangle, x);
        Canvas.SetTop(rectangle, y);
        canvas.Children.Add(rectangle);
    }

    private static void AddEllipse(Canvas canvas, double x, double y, double width, double height)
    {
        var ellipse = new Ellipse
        {
            Width = width,
            Height = height,
            StrokeThickness = 2,
            Fill = Brushes.Transparent
        };
        BindStroke(ellipse);
        Canvas.SetLeft(ellipse, x);
        Canvas.SetTop(ellipse, y);
        canvas.Children.Add(ellipse);
    }

    private static void BindStroke(Shape shape)
    {
        shape.SetBinding(Shape.StrokeProperty, new Binding(nameof(Button.Foreground))
        {
            RelativeSource = new RelativeSource(RelativeSourceMode.FindAncestor, typeof(Button), 1)
        });
    }

    public static Border GlassPanel(UIElement child, double radius = 8, Thickness? padding = null)
    {
        return new Border
        {
            Background = Brush(232, 250, 250, 250),
            BorderBrush = Brush(168, 255, 255, 255),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(radius),
            Padding = padding ?? new Thickness(5),
            Effect = Shadow(78, 42, 14),
            Child = child
        };
    }

    private static ControlTemplate RoundButtonTemplate(double radius)
    {
        var template = new ControlTemplate(typeof(Button));
        var border = new FrameworkElementFactory(typeof(Border));
        border.Name = "Root";
        border.SetValue(Border.CornerRadiusProperty, new CornerRadius(radius));
        border.SetBinding(Border.BackgroundProperty, new Binding(nameof(Button.Background)) { RelativeSource = RelativeSource.TemplatedParent });
        var presenter = new FrameworkElementFactory(typeof(ContentPresenter));
        presenter.SetValue(FrameworkElement.HorizontalAlignmentProperty, HorizontalAlignment.Center);
        presenter.SetValue(FrameworkElement.VerticalAlignmentProperty, VerticalAlignment.Center);
        border.AppendChild(presenter);
        template.VisualTree = border;

        var hover = new Trigger { Property = UIElement.IsMouseOverProperty, Value = true };
        hover.Setters.Add(new Setter(Control.BackgroundProperty, Brush(34, 31, 41, 51)));
        hover.Setters.Add(new Setter(Control.ForegroundProperty, Brush(31, 41, 51)));
        template.Triggers.Add(hover);

        var pressed = new Trigger { Property = ButtonBase.IsPressedProperty, Value = true };
        pressed.Setters.Add(new Setter(Control.BackgroundProperty, Brush(58, 31, 41, 51)));
        pressed.Setters.Add(new Setter(Control.ForegroundProperty, Brush(31, 41, 51)));
        template.Triggers.Add(pressed);

        return template;
    }
}
