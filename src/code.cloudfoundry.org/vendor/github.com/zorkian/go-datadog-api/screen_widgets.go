package datadog

type TextSize struct {
	Size int
	Auto bool
}

type TileDef struct {
	Events   []TileDefEvent      `json:"events"`
	Requests []TimeseriesRequest `json:"requests"`
	Viz      string              `json:"viz"`
}

type TimeseriesRequest struct {
	Query              string                 `json:"q"`
	Type               string                 `json:"type,omitempty"`
	ConditionalFormats []ConditionalFormat    `json:"conditional_formats,omitempty"`
	Style              TimeseriesRequestStyle `json:"style,omitempty"`
}

type TimeseriesRequestStyle struct {
	Palette string `json:"palette"`
}

type TileDefEvent struct {
	Query string `json:"q"`
}

type AlertValueWidget struct {
	TitleSize    int    `json:"title_size"`
	Title        bool   `json:"title"`
	TitleAlign   string `json:"title_align"`
	TextAlign    string `json:"text_align"`
	TitleText    string `json:"title_text"`
	Precision    int    `json:"precision"`
	AlertId      int    `json:"alert_id"`
	Timeframe    string `json:"timeframe"`
	AddTimeframe bool   `json:"add_timeframe"`
	Y            int    `json:"y"`
	X            int    `json:"x"`
	TextSize     string `json:"text_size"`
	Height       int    `json:"height"`
	Width        int    `json:"width"`
	Type         string `json:"type"`
	Unit         string `json:"unit"`
}

type ChangeWidget struct {
	TitleSize  int     `json:"title_size"`
	Title      bool    `json:"title"`
	TitleAlign string  `json:"title_align"`
	TitleText  string  `json:"title_text"`
	Height     int     `json:"height"`
	Width      int     `json:"width"`
	X          int     `json:"y"`
	Y          int     `json:"x"`
	Aggregator string  `json:"aggregator"`
	TileDef    TileDef `json:"tile_def"`
}

type GraphWidget struct {
	TitleSize  int     `json:"title_size"`
	Title      bool    `json:"title"`
	TitleAlign string  `json:"title_align"`
	TitleText  string  `json:"title_text"`
	Height     int     `json:"height"`
	Width      int     `json:"width"`
	X          int     `json:"y"`
	Y          int     `json:"x"`
	Type       string  `json:"type"`
	Timeframe  string  `json:"timeframe"`
	LegendSize int     `json:"legend_size"`
	Legend     bool    `json:"legend"`
	TileDef    TileDef `json:"tile_def"`
}

type EventTimelineWidget struct {
	TitleSize  int    `json:"title_size"`
	Title      bool   `json:"title"`
	TitleAlign string `json:"title_align"`
	TitleText  string `json:"title_text"`
	Height     int    `json:"height"`
	Width      int    `json:"width"`
	X          int    `json:"y"`
	Y          int    `json:"x"`
	Type       string `json:"type"`
	Timeframe  string `json:"timeframe"`
	Query      string `json:"query"`
}

type AlertGraphWidget struct {
	TitleSize    int    `json:"title_size"`
	VizType      string `json:"timeseries"`
	Title        bool   `json:"title"`
	TitleAlign   string `json:"title_align"`
	TitleText    string `json:"title_text"`
	Height       int    `json:"height"`
	Width        int    `json:"width"`
	X            int    `json:"y"`
	Y            int    `json:"x"`
	AlertId      int    `json:"alert_id"`
	Timeframe    string `json:"timeframe"`
	Type         string `json:"type"`
	AddTimeframe bool   `json:"add_timeframe"`
}

type HostMapWidget struct {
	TitleSize  int     `json:"title_size"`
	Title      bool    `json:"title"`
	TitleAlign string  `json:"title_align"`
	TitleText  string  `json:"title_text"`
	Height     int     `json:"height"`
	Width      int     `json:"width"`
	X          int     `json:"y"`
	Y          int     `json:"x"`
	Query      string  `json:"query"`
	Timeframe  string  `json:"timeframe"`
	LegendSize int     `json:"legend_size"`
	Type       string  `json:"type"`
	Legend     bool    `json:"legend"`
	TileDef    TileDef `json:"tile_def"`
}

type CheckStatusWidget struct {
	TitleSize  int    `json:"title_size"`
	Title      bool   `json:"title"`
	TitleAlign string `json:"title_align"`
	TextAlign  string `json:"text_align"`
	TitleText  string `json:"title_text"`
	Height     int    `json:"height"`
	Width      int    `json:"width"`
	X          int    `json:"y"`
	Y          int    `json:"x"`
	Tags       string `json:"tags"`
	Timeframe  string `json:"timeframe"`
	TextSize   string `json:"text_size"`
	Type       string `json:"type"`
	Check      string `json:"check"`
	Group      string `json:"group"`
	Grouping   string `json:"grouping"`
}

type IFrameWidget struct {
	TitleSize  int    `json:"title_size"`
	Title      bool   `json:"title"`
	Url        string `json:"url"`
	TitleAlign string `json:"title_align"`
	TitleText  string `json:"title_text"`
	Height     int    `json:"height"`
	Width      int    `json:"width"`
	X          int    `json:"y"`
	Y          int    `json:"x"`
	Type       string `json:"type"`
}

type NoteWidget struct {
	TitleSize    int    `json:"title_size"`
	Title        bool   `json:"title"`
	RefreshEvery int    `json:"refresh_every"`
	TickPos      string `json:"tick_pos"`
	TitleAlign   string `json:"title_align"`
	TickEdge     string `json:"tick_edge"`
	TextAlign    string `json:"text_align"`
	TitleText    string `json:"title_text"`
	Height       int    `json:"height"`
	Color        string `json:"bgcolor"`
	Html         string `json:"html"`
	Y            int    `json:"y"`
	X            int    `json:"x"`
	FontSize     int    `json:"font_size"`
	Tick         bool   `json:"tick"`
	Note         string `json:"type"`
	Width        int    `json:"width"`
	AutoRefresh  bool   `json:"auto_refresh"`
}

type TimeseriesWidget struct {
	Height     int      `json:"height"`
	Legend     bool     `json:"legend"`
	TileDef    TileDef  `json:"tile_def"`
	Timeframe  string   `json:"timeframe"`
	Title      bool     `json:"title"`
	TitleAlign string   `json:"title_align"`
	TitleSize  TextSize `json:"title_size"`
	TitleText  string   `json:"title_text"`
	Type       string   `json:"type"`
	Width      int      `json:"width"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
}

type QueryValueWidget struct {
	Timeframe           string              `json:"timeframe"`
	TimeframeAggregator string              `json:"aggr"`
	Aggregator          string              `json:"aggregator"`
	CalcFunc            string              `json:"calc_func"`
	ConditionalFormats  []ConditionalFormat `json:"conditional_formats"`
	Height              int                 `json:"height"`
	IsValidQuery        bool                `json:"is_valid_query,omitempty"`
	Metric              string              `json:"metric"`
	MetricType          string              `json:"metric_type"`
	Precision           int                 `json:"precision"`
	Query               string              `json:"query"`
	ResultCalcFunc      string              `json:"res_calc_func"`
	Tags                []string            `json:"tags"`
	TextAlign           string              `json:"text_align"`
	TextSize            TextSize            `json:"text_size"`
	Title               bool                `json:"title"`
	TitleAlign          string              `json:"title_align"`
	TitleSize           TextSize            `json:"title_size"`
	TitleText           string              `json:"title_text"`
	Type                string              `json:"type"`
	Unit                string              `json:"auto"`
	Width               int                 `json:"width"`
	X                   int                 `json:"x"`
	Y                   int                 `json:"y"`
}
type ConditionalFormat struct {
	Color      string `json:"color"`
	Comparator string `json:"comparator"`
	Inverted   bool   `json:"invert"`
	Value      int    `json:"value"`
}

type ToplistWidget struct {
	Height     int      `json:"height"`
	Legend     bool     `json:"legend"`
	LegendSize int      `json:"legend_size"`
	TileDef    TileDef  `json:"tile_def"`
	Timeframe  string   `json:"timeframe"`
	Title      bool     `json:"title"`
	TitleAlign string   `json:"title_align"`
	TitleSize  TextSize `json:"title_size"`
	TitleText  string   `json:"title_text"`
	Type       string   `json:"type"`
	Width      int      `json:"width"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
}

type EventStreamWidget struct {
	EventSize  string   `json:"event_size"`
	Height     int      `json:"height"`
	Query      string   `json:"query"`
	Timeframe  string   `json:"timeframe"`
	Title      bool     `json:"title"`
	TitleAlign string   `json:"title_align"`
	TitleSize  TextSize `json:"title_size"`
	TitleText  string   `json:"title_text"`
	Type       string   `json:"type"`
	Width      int      `json:"width"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
}

type FreeTextWidget struct {
	Color     string `json:"color,omitempty"`
	FontSize  string `json:"font_size,omitempty"`
	Height    int    `json:"height,omitempty"`
	Text      string `json:"text"`
	TextAlign string `json:"text_align"`
	Type      string `json:"type"`
	Width     int    `json:"width"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
}

type ImageWidget struct {
	Height     int      `json:"height"`
	Sizing     string   `json:"sizing"`
	Title      bool     `json:"title"`
	TitleAlign string   `json:"title_align"`
	TitleSize  TextSize `json:"title_size"`
	TitleText  string   `json:"title_text"`
	Type       string   `json:"type"`
	Url        string   `json:"url"`
	Width      int      `json:"width"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
}
