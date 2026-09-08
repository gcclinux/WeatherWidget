//go:build darwin

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// navButton represents a single navigation button in the custom sidebar.
type navButton struct {
	widget.BaseWidget
	icon     fyne.Resource
	text     string
	selected bool
	onTap    func()
}

func newNavButton(icon fyne.Resource, text string, onTap func()) *navButton {
	b := &navButton{
		icon:  icon,
		text:  text,
		onTap: onTap,
	}
	b.ExtendBaseWidget(b)
	return b
}

func (b *navButton) Tapped(_ *fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *navButton) SetSelected(selected bool) {
	b.selected = selected
	b.Refresh()
}

func (b *navButton) CreateRenderer() fyne.WidgetRenderer {
	iconImg := canvas.NewImageFromResource(b.icon)
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(24, 24))

	label := canvas.NewText(b.text, color.NRGBA{R: 30, G: 41, B: 59, A: 255})
	label.TextSize = 12
	label.Alignment = fyne.TextAlignCenter

	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 8

	indicator := canvas.NewRectangle(color.NRGBA{R: 37, G: 99, B: 235, A: 255})
	indicator.CornerRadius = 2
	indicator.Hide()

	return &navButtonRenderer{
		button:    b,
		icon:      iconImg,
		label:     label,
		bg:        bg,
		indicator: indicator,
	}
}

type navButtonRenderer struct {
	button    *navButton
	icon      *canvas.Image
	label     *canvas.Text
	bg        *canvas.Rectangle
	indicator *canvas.Rectangle
}

func (r *navButtonRenderer) Destroy() {}

func (r *navButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)

	iconSize := float32(24)
	padding := float32(8)

	// Icon centered horizontally, near top
	iconX := (size.Width - iconSize) / 2
	iconY := padding
	r.icon.Move(fyne.NewPos(iconX, iconY))
	r.icon.Resize(fyne.NewSize(iconSize, iconSize))

	// Label below icon
	labelHeight := r.label.MinSize().Height
	labelY := iconY + iconSize + 4
	r.label.Move(fyne.NewPos(0, labelY))
	r.label.Resize(fyne.NewSize(size.Width, labelHeight))

	// Indicator on the right edge when selected
	indicatorWidth := float32(3)
	r.indicator.Move(fyne.NewPos(size.Width-indicatorWidth, 8))
	r.indicator.Resize(fyne.NewSize(indicatorWidth, size.Height-16))
}

func (r *navButtonRenderer) MinSize() fyne.Size {
	labelSize := r.label.MinSize()
	return fyne.NewSize(80, 24+labelSize.Height+20)
}

func (r *navButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon, r.label, r.indicator}
}

func (r *navButtonRenderer) Refresh() {
	if r.button.selected {
		r.bg.FillColor = color.NRGBA{R: 239, G: 246, B: 255, A: 255}
		r.label.Color = color.NRGBA{R: 37, G: 99, B: 235, A: 255}
		r.label.TextStyle.Bold = true
		r.indicator.Show()
	} else {
		r.bg.FillColor = color.Transparent
		r.label.Color = color.NRGBA{R: 100, G: 116, B: 139, A: 255}
		r.label.TextStyle.Bold = false
		r.indicator.Hide()
	}
	r.label.Text = r.button.text
	r.bg.Refresh()
	r.label.Refresh()
	r.indicator.Refresh()
}

// customSettingsNav creates a custom navigation sidebar for macOS that doesn't
// use AppTabs, avoiding the hover text disappearing bug.
type customSettingsNav struct {
	buttons   []*navButton
	contents  []fyne.CanvasObject
	selected  int
	container *fyne.Container
	content   *fyne.Container
}

// newCustomSettingsNav creates a navigation container with sidebar buttons and content area.
// This is used on macOS to avoid the AppTabs hover bug where tab labels disappear.
func newCustomSettingsNav(items []struct {
	icon    fyne.Resource
	text    string
	content fyne.CanvasObject
}) (*fyne.Container, func(int)) {
	nav := &customSettingsNav{
		selected: 0,
		contents: make([]fyne.CanvasObject, len(items)),
	}

	// Create content container that will hold the active content
	nav.content = container.NewStack()

	// Create buttons
	buttonList := container.NewVBox()
	for i, item := range items {
		idx := i
		nav.contents[i] = item.content

		btn := newNavButton(item.icon, item.text, func() {
			nav.selectIndex(idx)
		})
		nav.buttons = append(nav.buttons, btn)
		buttonList.Add(btn)
	}

	// Select first item
	if len(nav.buttons) > 0 {
		nav.buttons[0].SetSelected(true)
		nav.content.Add(nav.contents[0])
	}

	// Create sidebar with background
	sidebarBg := canvas.NewRectangle(color.NRGBA{R: 250, G: 250, B: 252, A: 255})
	sidebarContent := container.NewVBox(
		container.NewPadded(buttonList),
	)
	sidebar := container.NewStack(sidebarBg, sidebarContent)

	// Create divider
	divider := canvas.NewRectangle(color.NRGBA{R: 226, G: 232, B: 240, A: 255})
	divider.SetMinSize(fyne.NewSize(1, 0))

	// Main layout: sidebar | divider | content
	nav.container = container.NewBorder(nil, nil, container.NewHBox(sidebar, divider), nil, nav.content)

	selectFunc := func(index int) {
		nav.selectIndex(index)
	}

	return nav.container, selectFunc
}

func (nav *customSettingsNav) selectIndex(index int) {
	if index < 0 || index >= len(nav.buttons) || index == nav.selected {
		return
	}

	// Deselect old
	nav.buttons[nav.selected].SetSelected(false)

	// Select new
	nav.selected = index
	nav.buttons[index].SetSelected(true)

	// Update content
	nav.content.RemoveAll()
	nav.content.Add(nav.contents[index])
	nav.content.Refresh()
}

// buildSettingsNavDarwin creates a macOS-specific settings navigation that avoids
// the AppTabs hover bug. Returns the navigation container and a function to select
// tabs by index.
func buildSettingsNavDarwin(items []struct {
	icon    fyne.Resource
	text    string
	content fyne.CanvasObject
}) (*fyne.Container, func(int)) {
	return newCustomSettingsNav(items)
}

// useDarwinNav returns true on macOS to indicate custom navigation should be used.
func useDarwinNav() bool {
	return true
}
