package extensions

import "fmt"

type CssComponents struct{}

func (c CssComponents) Name() string {
	return "css-components"
}

func (c CssComponents) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("css-components: context or data is nil")
	}

	if err := c.renderTemplates(ctx); err != nil {
		return fmt.Errorf("css-components: failed to render templates: %w", err)
	}

	return nil
}

func (c CssComponents) Dependencies() []string {
	return nil
}

func (c CssComponents) renderTemplates(ctx *Context) error {
	return c.renderComponentTemplates(ctx)
}

func (c CssComponents) renderComponentTemplates(ctx *Context) error {
	componentTemplates := map[string]string{
		"css_themes.tmpl":                  "css/themes.css",
		"css_utilities.tmpl":               "css/utilities.css",
		"css_components.tmpl":              "css/components.css",
		"views_components_toast.tmpl":      "views/components/toast.templ",
		"views_examples_accordian.tmpl":    "views/examples/accordion.html",
		"views_examples_alerts.tmpl":       "views/examples/alerts.html",
		"views_examples_aspect_ratio.tmpl": "views/examples/aspect_ratio.html",
		"views_examples_avatar.tmpl":       "views/examples/avatar.html",
		"views_examples_badges.tmpl":       "views/examples/badges.html",
		"views_examples_breadcrump.tmpl":   "views/examples/breadcrumb.html",
		"views_examples_buttons.tmpl":      "views/examples/buttons.html",
		"views_examples_calendar.tmpl":     "views/examples/calendar.html",
		"views_examples_card.tmpl":         "views/examples/card.html",
		"views_examples_carousel.tmpl":     "views/examples/carousel.html",
		"views_examples_checkbox.tmpl":     "views/examples/checkbox.html",
		"views_examples_code.tmpl":         "views/examples/code.html",
		"views_examples_collapsible.tmpl":  "views/examples/collapsible.html",
		"views_examples_combobox.tmpl":     "views/examples/combobox.html",
		"views_examples_copy_button.tmpl":  "views/examples/copy_button.html",
		"views_examples_data_input.tmpl":   "views/examples/date_input.html",
		"views_examples_dialog.tmpl":       "views/examples/dialog.html",
		"views_examples_dropdown.tmpl":     "views/examples/dropdown.html",
		"views_examples_empty_state.tmpl":  "views/examples/empty_state.html",
		"views_examples_input.tmpl":        "views/examples/input.html",
		"views_examples_input_group.tmpl":  "views/examples/input_group.html",
		"views_examples_input_otp.tmpl":    "views/examples/input_otp.html",
		"views_examples_kdb.tmpl":          "views/examples/kbd.html",
		"views_examples_menu.tmpl":         "views/examples/menu.html",
		"views_examples_pagination.tmpl":   "views/examples/pagination.html",
		"views_examples_popover.tmpl":      "views/examples/popover.html",
		"views_examples_progress.tmpl":     "views/examples/progress.html",
		"views_examples_radio.tmpl":        "views/examples/radio.html",
		"views_examples_rating.tmpl":       "views/examples/rating.html",
		"views_examples_select.tmpl":       "views/examples/select.html",
		"views_examples_separator.tmpl":    "views/examples/separator.html",
		"views_examples_sheet.tmpl":        "views/examples/sheet.html",
		"views_examples_skeleton.tmpl":     "views/examples/skeleton.html",
		"views_examples_slider.tmpl":       "views/examples/slider.html",
		"views_examples_spinner.tmpl":      "views/examples/spinner.html",
		"views_examples_stats.tmpl":        "views/examples/stats.html",
		"views_examples_steps.tmpl":        "views/examples/steps.html",
		"views_examples_switch.tmpl":       "views/examples/switch.html",
		"views_examples_table.tmpl":        "views/examples/table.html",
		"views_examples_tabs.tmpl":         "views/examples/tabs.html",
		"views_examples_tabs_input.tmpl":   "views/examples/tabs_input.html",
		"views_examples_textarea.tmpl":     "views/examples/textarea.html",
		"views_examples_time_input.tmpl":   "views/examples/time_input.html",
		"views_examples_toast.tmpl":        "views/examples/toast.html",
		"views_examples_tooltip.tmpl":      "views/examples/tooltip.html",
	}

	for tmpl, target := range componentTemplates {
		templatePath := fmt.Sprintf("templates/css-components/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
