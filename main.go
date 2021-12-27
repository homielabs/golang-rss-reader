package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mmcdole/gofeed"
	"github.com/spf13/viper"
)

const (
	headerHeight               = 3
	footerHeight               = 3
	useHighPerformanceRenderer = false
)

type model struct {
	feedSlice         []gofeed.Feed
	feedSliceIndex    int
	feed              gofeed.Feed
	feedIndex         int
	ready             bool
	viewport          viewport.Model
	help              help.Model
	markdownConverter md.Converter
	// config-based
	accent          string
	textColor       string
	backgroundColor string
	horzPadding     int
	vertPadding     int
	fetchTimeout    int
}

type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Help  key.Binding
	Quit  key.Binding
}

var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/left", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/right", "move right"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc/<C-c>", "quit"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right}, // first column
		{k.Help, k.Quit},                // second column
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func renderContent(content string, markdownConverter md.Converter) string {
	var err error
	// unescape HTML entities
	content = html.UnescapeString(content)
	// pass to HTML -> markdown converter (oops)
	content, err = markdownConverter.ConvertString(content)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// pass markdown content to glamour
	content, err = glamour.Render(content, "dark")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	return content
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	rerender := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, defaultKeyMap.Left):
			if m.feedIndex > 0 {
				m.feedIndex--
				rerender = true
			}
		case key.Matches(msg, defaultKeyMap.Right):
			if m.feedIndex < getFeedLengthOrZero(m.feedSlice[m.feedSliceIndex]) {
				m.feedIndex++
				rerender = true
			}
		case key.Matches(msg, defaultKeyMap.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, defaultKeyMap.Quit):
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		// set the width on the help menu if necessary (truncate if required)
		m.help.Width = msg.Width

		verticalMargins := headerHeight + footerHeight

		if !m.ready {
			// Since this program is using the full size of the viewport we need
			// to wait until we've received the window dimensions before we
			// can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.Model{
				Width:  msg.Width,
				Height: msg.Height - verticalMargins,
			}
			m.viewport.HighPerformanceRendering = useHighPerformanceRenderer

			// render content
			rerender = true

			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

		if useHighPerformanceRenderer {
			// Render (or re-render) the whole viewport. Necessary both to
			// initialize the viewport and when the window is resized.
			//
			// This is needed for high-performance rendering only.
			cmds = append(cmds, viewport.Sync(m.viewport))
		}
	}

	if rerender {
		// the content that will be rendered
		var content string
		// if the feed is empty
		// this case would also handle where we index out of bounds, but that case
		// should not be handled here; it should already be handled where we attempt
		// to increment/decrement the feedIndex
		if m.feedIndex >= m.feedSlice[m.feedSliceIndex].Len() {
			content = "No content here!"
		} else {
			item := m.feedSlice[m.feedSliceIndex].Items[m.feedIndex]
			content = renderContent(
				// inject a <hr> so the HTML -> MD converter will render the break
				item.Description+"<hr>"+item.Content, m.markdownConverter,
			)
		}
		m.viewport.SetContent(content)
		m.ready = true
	}

	// Because we're using the viewport's default update function (with pager-
	// style navigation) it's important that the viewport's update function:
	//
	// * Receives messages from the Bubble Tea runtime
	// * Returns commands to the Bubble Tea runtime
	//
	m.viewport, cmd = m.viewport.Update(msg)
	if useHighPerformanceRenderer {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func getFeedLengthOrZero(feed gofeed.Feed) int {
	if feed.Len()-1 > 0 {
		return feed.Len() - 1
	} else {
		return 0
	}
}

func assembleHeader(title string, m model) string {
	return lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color(m.accent)).
		Foreground(lipgloss.Color(m.textColor)).
		PaddingLeft(m.horzPadding).
		PaddingRight(m.horzPadding).
		PaddingTop(m.vertPadding).
		PaddingBottom(m.vertPadding).
		Render(title)
}

func assembleFooter(authors []string, publishedTime time.Time, m model) string {
	var genericHorzPaddedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(m.backgroundColor)).
		PaddingLeft(m.horzPadding).
		PaddingRight(m.horzPadding)

	var progressFormattedStr = genericHorzPaddedStyle.Copy().
		Bold(true).
		Background(lipgloss.Color(m.accent)).
		Foreground(lipgloss.Color(m.textColor)).
		Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))

	var articleCounterFormattedStr = genericHorzPaddedStyle.
		Render(
			fmt.Sprintf("%d/%d articles", m.feedIndex, getFeedLengthOrZero(m.feedSlice[m.feedSliceIndex])),
		)

	var authorsFormattedStr = genericHorzPaddedStyle.Copy().
		Align(lipgloss.Right).
		BorderLeft(true).
		BorderLeftForeground(lipgloss.Color(m.textColor)).
		Render(strings.Join(authors, ", "))

	var timeFormattedStr = genericHorzPaddedStyle.Copy().
		Align(lipgloss.Right).
		BorderLeft(true).
		BorderLeftForeground(lipgloss.Color(m.textColor)).
		Render(
			"Last updated " + publishedTime.Local().Format("2006-01-02 15:04:05 MST"),
		)

	// since the max width is passed into this function, create some whitespace
	// to fill out the extra space.
	consumedWidth := lipgloss.Width(progressFormattedStr) +
		lipgloss.Width(articleCounterFormattedStr) +
		lipgloss.Width(authorsFormattedStr) +
		lipgloss.Width(timeFormattedStr)
	// the empty str in Render() will be turned into spaces as per bubble's
	// whitespace docs.
	spacerStr := lipgloss.NewStyle().
		Background(lipgloss.Color(m.backgroundColor)).
		Width(m.viewport.Width - consumedWidth).
		Render("")

	return lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		progressFormattedStr,
		articleCounterFormattedStr,
		spacerStr,
		authorsFormattedStr,
		timeFormattedStr,
	)
}

func (m model) View() string {
	if !m.ready {
		return "\n Loading content"
	}

	// return the help first if that's what was requested
	if m.help.ShowAll {
		helpView := m.help.View(defaultKeyMap)
		return helpView
	}

	if m.feedIndex < m.feedSlice[m.feedSliceIndex].Len() {
		item := m.feedSlice[m.feedSliceIndex].Items[m.feedIndex]
		var authorNames []string
		for _, x := range item.Authors {
			authorNames = append(authorNames, x.Name)
		}

		// try to pick a sensible "last updated" date
		lastUpdatedDate := time.Unix(0, 0) // default to unix epoch
		if item.PublishedParsed != nil {
			lastUpdatedDate = *item.PublishedParsed
		}
		if item.UpdatedParsed != nil {
			lastUpdatedDate = *item.UpdatedParsed
		}

		return fmt.Sprintf("%s\n%s\n%s",
			assembleHeader(item.Title, m),
			m.viewport.View(),
			assembleFooter(authorNames, lastUpdatedDate, m),
		)
	} else {
		return fmt.Sprintf("%s\n%s\n%s",
			assembleHeader("No content", m),
			m.viewport.View(),
			assembleFooter(nil, time.Unix(0, 0), m),
		)
	}
}

func main() {
	// write everything to logfile
	logFile, err := os.OpenFile(
		"golang-rss-client.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644,
	)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close the logfile after we exit
	defer logFile.Close()
	// switch over to logFile output
	log.SetOutput(logFile)

	// defaults for color in reader
	viper.SetDefault("accent", "33")
	viper.SetDefault("textColor", "15")
	viper.SetDefault("backgroundColor", "233")
	viper.SetDefault("horzPadding", 2)
	viper.SetDefault("vertPadding", 0)
	viper.SetDefault("fetchTimeout", 15)
	defaultFeedUrls := [1]string{"https://github.com/homielabs.atom"}
	log.Println(defaultFeedUrls)
	viper.SetDefault("feedUrls", defaultFeedUrls)

	// config file locations
	viper.SetConfigName("golang-rss-client.yml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/golang-rss-client/")
	viper.AddConfigPath("$HOME/golang-rss-client/")
	viper.AddConfigPath(".")

	// read env vars
	viper.SetEnvPrefix("golangrssclient")
	viper.BindEnv("accent")
	viper.BindEnv("textColor")
	viper.BindEnv("backgroundColor")
	viper.BindEnv("horzPadding")
	viper.BindEnv("vertPadding")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error since we have defaults
			log.Println("Found no configs on disk")
		} else {
			// Config file was found but another error was produced
			log.Fatal(err)
			os.Exit(1)
		}
	}

	log.Println(viper.AllSettings())

	// parse the feeds
	feedUrls := viper.GetStringSlice("feedUrls")
	var feedSlice []gofeed.Feed

	feedParser := gofeed.NewParser()
	for _, feedUrl := range feedUrls {
		// create a timeout
		fetchTimeout := viper.GetInt("fetchTimeout")
		ctx, cancel := context.WithTimeout(
			context.Background(), time.Duration(fetchTimeout)*time.Second,
		)
		defer cancel()
		// parse the feed
		feed, err := feedParser.ParseURLWithContext(feedUrl, ctx)
		// bug out if necessary
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		feedSlice = append(feedSlice, *feed)
	}

	// define a starter model
	starter_model := model{
		feedIndex:         0,
		help:              help.NewModel(),
		markdownConverter: *md.NewConverter("", true, nil),
		accent:            viper.GetString("accent"),
		textColor:         viper.GetString("textColor"),
		backgroundColor:   viper.GetString("backgroundColor"),
		horzPadding:       viper.GetInt("horzPadding"),
		vertPadding:       viper.GetInt("vertPadding"),
		fetchTimeout:      viper.GetInt("fetchTimeout"),
		feedSlice:         feedSlice,
		feedSliceIndex:    0,
	}
	// create the bubbletea program with the starter model
	p := tea.NewProgram(
		starter_model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if err := p.Start(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
