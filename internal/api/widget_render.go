package api

import (
    "git.zk35.de/secalpha/homeport/internal/db"
    "git.zk35.de/secalpha/homeport/internal/i18n"
)

// WidgetRenderData wraps a Widget with a translator for standalone partial renders.
type WidgetRenderData struct {
    db.Widget
    T func(string) string
}

func newWidgetRender(w db.Widget, lang string) WidgetRenderData {
    return WidgetRenderData{Widget: w, T: i18n.T(lang)}
}
