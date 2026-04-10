package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// rightClickOverlay is a transparent widget placed over the window content
// that intercepts secondary (right-click) taps to show a popup menu.
type rightClickOverlay struct {
	widget.BaseWidget
	menu    *fyne.Menu
	content fyne.CanvasObject
}

// newRightClickOverlay wraps content with a right-click handler that shows
// the given menu as a popup on the parent canvas.
func newRightClickOverlay(content fyne.CanvasObject, menu *fyne.Menu) *rightClickOverlay {
	o := &rightClickOverlay{
		menu:    menu,
		content: content,
	}
	o.ExtendBaseWidget(o)
	return o
}

// CreateRenderer returns a renderer that simply draws the wrapped content.
func (o *rightClickOverlay) CreateRenderer() fyne.WidgetRenderer {
	return &rightClickRenderer{overlay: o}
}

// TappedSecondary shows the context menu popup at the tap position.
func (o *rightClickOverlay) TappedSecondary(ev *fyne.PointEvent) {
	c := fyne.CurrentApp().Driver().CanvasForObject(o)
	if c == nil {
		return
	}
	widget.ShowPopUpMenuAtPosition(o.menu, c, ev.AbsolutePosition)
}

// Tapped is required by the Tappable interface but does nothing.
func (o *rightClickOverlay) Tapped(_ *fyne.PointEvent) {}

// rightClickRenderer renders the wrapped content filling the overlay size.
type rightClickRenderer struct {
	overlay *rightClickOverlay
}

func (r *rightClickRenderer) Layout(size fyne.Size) {
	r.overlay.content.Resize(size)
	r.overlay.content.Move(fyne.NewPos(0, 0))
}

func (r *rightClickRenderer) MinSize() fyne.Size {
	return r.overlay.content.MinSize()
}

func (r *rightClickRenderer) Refresh() {}

func (r *rightClickRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.overlay.content}
}

func (r *rightClickRenderer) Destroy() {}
