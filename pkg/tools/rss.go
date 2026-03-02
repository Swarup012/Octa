package tools

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	_ "modernc.org/sqlite"

	pkgdb "github.com/Swarup012/solo/pkg/db"
	"github.com/Swarup012/solo/pkg/logger"
)

const maxItemsPerFeed = 50

type RSSFeedItem struct {
	ID          string
	FeedID      string
	Title       string
	Description string
	Content     string
	Link        string
	Published   string
	Read        bool
}

type RSSFeed struct {
	ID       string
	URL      string
	Name     string
	Priority bool
	LastSync time.Time
}

type RSSFetcher struct {
	db *sql.DB
}

type RSSFeedTool struct {
	db         *sql.DB
	initOnce   sync.Once
	initErr    error
	dbPath     string
}

func NewRSSFeedTool(dbPath string) *RSSFeedTool {
	return &RSSFeedTool{
		dbPath: dbPath,
	}
}

func (t *RSSFeedTool) Name() string {
	return "rss"
}

func (t *RSSFeedTool) Description() string {
	return "Manage RSS feeds and read articles. Actions: subscribe, unsubscribe, list_feeds, read, unread, mark_read"
}

func (t *RSSFeedTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: subscribe, unsubscribe, list_feeds, read, unread, mark_read",
				"enum":        []string{"subscribe", "unsubscribe", "list_feeds", "read", "unread", "mark_read"},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Feed URL for subscribe action",
			},
			"feed_id": map[string]any{
				"type":        "string",
				"description": "Feed ID for unsubscribe action",
			},
			"priority": map[string]any{
				"type":        "boolean",
				"description": "Mark feed as priority (string 'true' or bool)",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "Number of items to read (can be string from LLM)",
			},
			"item_id": map[string]any{
				"type":        "string",
				"description": "Item ID for mark_read action",
			},
		},
		"required": []string{"action"},
	}
}

func (t *RSSFeedTool) initDB() error {
	t.initOnce.Do(func() {
		sharedDB, err := pkgdb.Get(t.dbPath)
		if err != nil {
			t.initErr = fmt.Errorf("failed to open database: %w", err)
			return
		}
		db := sharedDB

		// Create tables
		schema := `
		CREATE TABLE IF NOT EXISTS rss_feeds (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			priority BOOLEAN DEFAULT 0,
			last_sync TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS rss_items (
			id TEXT PRIMARY KEY,
			feed_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			content TEXT,
			link TEXT,
			published TIMESTAMP,
			read BOOLEAN DEFAULT 0,
			FOREIGN KEY(feed_id) REFERENCES rss_feeds(id)
		);
		CREATE INDEX IF NOT EXISTS idx_rss_items_feed ON rss_items(feed_id);
		CREATE INDEX IF NOT EXISTS idx_rss_items_read ON rss_items(read);
		`

		if _, err := db.ExecContext(context.Background(), schema); err != nil {
			t.initErr = err
			return
		}

		t.db = db
	})
	return t.initErr
}

func (t *RSSFeedTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	if err := t.initDB(); err != nil {
		return ErrorResult(fmt.Sprintf("Database initialization failed: %v", err)).WithError(err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "subscribe":
		return t.actionSubscribe(ctx, args)
	case "unsubscribe":
		return t.actionUnsubscribe(ctx, args)
	case "list_feeds":
		return t.actionListFeeds(ctx, args)
	case "read":
		return t.actionRead(ctx, args)
	case "unread":
		return t.actionUnread(ctx, args)
	case "mark_read":
		return t.actionMarkRead(ctx, args)
	default:
		return ErrorResult(fmt.Sprintf("Unknown action: %s", action))
	}
}

func (t *RSSFeedTool) actionSubscribe(ctx context.Context, args map[string]any) *ToolResult {
	feedURL, ok := args["url"].(string)
	if !ok || feedURL == "" {
		return ErrorResult("url is required for subscribe action")
	}

	priority := false
	if p, ok := args["priority"].(bool); ok {
		priority = p
	} else if p, ok := args["priority"].(string); ok {
		priority = strings.ToLower(p) == "true"
	}

	// Try to parse feed directly first; if it fails, auto-discover RSS URL
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		discovered, discErr := discoverFeedURL(ctx, feedURL)
		if discErr != nil {
			return ErrorResult(fmt.Sprintf("Failed to parse feed and could not auto-discover RSS URL: %v", err)).WithError(err)
		}
		feedURL = discovered
		feed, err = fp.ParseURLWithContext(feedURL, ctx)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to parse discovered feed: %v", err)).WithError(err)
		}
	}

	feedID := rssSlug(feedURL)
	feedName := rssNameFromURL(feedURL)
	if feed.Title != "" {
		feedName = feed.Title
	}

	// Insert into database
	_, err = t.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO rss_feeds (id, url, name, priority, last_sync) VALUES (?, ?, ?, ?, ?)`,
		feedID, feedURL, feedName, priority, time.Now(),
	)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to save feed: %v", err)).WithError(err)
	}

	// Fetch and save articles
	if err := t.fetchFeedNow(ctx, feedID, feedURL); err != nil {
		logger.DebugCF("rss", "Failed to fetch feed articles", map[string]any{"error": err.Error()})
		// Don't fail subscription if fetch fails
	}

	return UserResult(fmt.Sprintf("Subscribed to '%s' (ID: %s)", feedName, feedID))
}

func (t *RSSFeedTool) actionUnsubscribe(ctx context.Context, args map[string]any) *ToolResult {
	feedID, ok := args["feed_id"].(string)
	if !ok || feedID == "" {
		return ErrorResult("feed_id is required for unsubscribe action")
	}

	// Delete feed and items
	_, err := t.db.ExecContext(ctx, `DELETE FROM rss_items WHERE feed_id = ?`, feedID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete items: %v", err)).WithError(err)
	}

	_, err = t.db.ExecContext(ctx, `DELETE FROM rss_feeds WHERE id = ?`, feedID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete feed: %v", err)).WithError(err)
	}

	return UserResult(fmt.Sprintf("Unsubscribed from feed %s", feedID))
}

func (t *RSSFeedTool) actionListFeeds(ctx context.Context, args map[string]any) *ToolResult {
	rows, err := t.db.QueryContext(ctx, `SELECT id, url, name, priority FROM rss_feeds ORDER BY name`)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to list feeds: %v", err)).WithError(err)
	}
	defer rows.Close()

	var result string
	count := 0
	for rows.Next() {
		var id, url, name string
		var priority bool
		if err := rows.Scan(&id, &url, &name, &priority); err != nil {
			continue
		}
		priorityMark := ""
		if priority {
			priorityMark = " [PRIORITY]"
		}
		result += fmt.Sprintf("• %s (%s)%s\n  %s\n", name, id, priorityMark, url)
		count++
	}

	if count == 0 {
		return UserResult("No feeds subscribed")
	}

	return UserResult(fmt.Sprintf("Subscribed feeds (%d):\n%s", count, result))
}

func (t *RSSFeedTool) actionRead(ctx context.Context, args map[string]any) *ToolResult {
	count := 10 // default
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	} else if c, ok := args["count"].(string); ok {
		if _, err := fmt.Sscanf(c, "%d", &count); err != nil {
			count = 10
		}
	}

	rows, err := t.db.QueryContext(ctx,
		`SELECT id, feed_id, title, description, link, published, read 
		 FROM rss_items WHERE read = 0 
		 ORDER BY published DESC LIMIT ?`,
		count,
	)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to read items: %v", err)).WithError(err)
	}
	defer rows.Close()

	var items []RSSFeedItem
	var itemIDs []string

	for rows.Next() {
		var item RSSFeedItem
		var published sql.NullString
		var read bool
		if err := rows.Scan(&item.ID, &item.FeedID, &item.Title, &item.Description, &item.Link, &published, &read); err != nil {
			continue
		}
		if published.Valid {
			item.Published = published.String
		}
		item.Read = read
		items = append(items, item)
		itemIDs = append(itemIDs, item.ID)
	}

	if len(items) == 0 {
		return UserResult("No unread articles")
	}

	// Mark all as read
	if len(itemIDs) > 0 {
		placeholders := strings.Repeat("?,", len(itemIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(itemIDs))
		for i, id := range itemIDs {
			args[i] = id
		}
		t.db.ExecContext(ctx, fmt.Sprintf(`UPDATE rss_items SET read = 1 WHERE id IN (%s)`, placeholders), args...)
	}

	var result string
	for i, item := range items {
		desc := truncate(stripHTML(item.Description), 200)
		result += fmt.Sprintf("%d. %s\n   %s\n   Link: %s\n\n", i+1, item.Title, desc, item.Link)
	}

	return UserResult(fmt.Sprintf("Latest articles (%d):\n\n%s", len(items), result))
}

func (t *RSSFeedTool) actionUnread(ctx context.Context, args map[string]any) *ToolResult {
	rows, err := t.db.QueryContext(ctx,
		`SELECT COUNT(*) FROM rss_items WHERE read = 0`,
	)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to count unread: %v", err)).WithError(err)
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		rows.Scan(&count)
	}

	return UserResult(fmt.Sprintf("You have %d unread articles", count))
}

func (t *RSSFeedTool) actionMarkRead(ctx context.Context, args map[string]any) *ToolResult {
	itemID, ok := args["item_id"].(string)
	if !ok || itemID == "" {
		return ErrorResult("item_id is required for mark_read action")
	}

	_, err := t.db.ExecContext(ctx, `UPDATE rss_items SET read = 1 WHERE id = ?`, itemID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to mark as read: %v", err)).WithError(err)
	}

	return SilentResult(fmt.Sprintf("Marked item %s as read", itemID))
}

func (t *RSSFeedTool) fetchFeedNow(ctx context.Context, feedID, feedURL string) error {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		return err
	}

	// Limit items to maxItemsPerFeed
	itemCount := maxItemsPerFeed
	if len(feed.Items) < itemCount {
		itemCount = len(feed.Items)
	}

	for i := 0; i < itemCount; i++ {
		item := feed.Items[i]
		itemID := rssItemID(feedID, item)
		description := item.Description

		published := ""
		if item.PublishedParsed != nil {
			published = item.PublishedParsed.Format(time.RFC3339)
		}

		link := item.Link
		if link == "" && len(item.Links) > 0 {
			link = item.Links[0]
		}

		_, err := t.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO rss_items (id, feed_id, title, description, content, link, published, read) 
			 VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
			itemID, feedID, item.Title, description, item.Content, link, published,
		)
		if err != nil {
			logger.DebugCF("rss", "Failed to insert item", map[string]any{"error": err.Error()})
		}
	}

	// Update last_sync
	_, err = t.db.ExecContext(ctx, `UPDATE rss_feeds SET last_sync = ? WHERE id = ?`, time.Now(), feedID)
	return err
}

// discoverFeedURL fetches a webpage and looks for RSS/Atom feed links in the HTML.
func discoverFeedURL(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "octa-rss-discovery/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1MB
	if err != nil {
		return "", err
	}

	html := string(body)

	// Look for <link rel="alternate" type="application/rss+xml" href="...">
	// or <link rel="alternate" type="application/atom+xml" href="...">
	re := regexp.MustCompile(`(?i)<link[^>]+type=["'](application/rss\+xml|application/atom\+xml)["'][^>]*href=["']([^"']+)["']|<link[^>]+href=["']([^"']+)["'][^>]+type=["'](application/rss\+xml|application/atom\+xml)["']`)
	matches := re.FindStringSubmatch(html)

	var feedHref string
	if len(matches) >= 3 && matches[2] != "" {
		feedHref = matches[2]
	} else if len(matches) >= 4 && matches[3] != "" {
		feedHref = matches[3]
	}

	if feedHref == "" {
		// Fallback: try common feed paths
		base, err := url.Parse(pageURL)
		if err != nil {
			return "", fmt.Errorf("no RSS feed found on page")
		}
		commonPaths := []string{"/rss", "/feed", "/rss.xml", "/feed.xml", "/atom.xml", "/rss/feed.xml"}
		client := &http.Client{Timeout: 5 * time.Second}
		for _, path := range commonPaths {
			candidate := base.Scheme + "://" + base.Host + path
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(r)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("no RSS feed found on page %s", pageURL)
	}

	// Resolve relative URLs
	base, err := url.Parse(pageURL)
	if err != nil {
		return feedHref, nil
	}
	feedParsed, err := url.Parse(feedHref)
	if err != nil {
		return feedHref, nil
	}
	return base.ResolveReference(feedParsed).String(), nil
}

// Helpers

func stripHTML(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	// Decode HTML entities
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&apos;", "'")
	return strings.TrimSpace(text)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func rssSlug(url string) string {
	hash := 0
	for _, c := range url {
		hash = ((hash << 5) - hash) + int(c)
	}
	return fmt.Sprintf("feed_%d", hash&0x7FFFFFFF)
}

func rssNameFromURL(url string) string {
	// Extract domain from URL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")
	if idx := strings.IndexByte(url, '/'); idx >= 0 {
		url = url[:idx]
	}
	return url
}

func rssItemID(feedID string, item *gofeed.Item) string {
	if item.GUID != "" {
		return fmt.Sprintf("%s_%s", feedID, item.GUID)
	}
	if item.Link != "" {
		hash := 0
		for _, c := range item.Link {
			hash = ((hash << 5) - hash) + int(c)
		}
		return fmt.Sprintf("%s_link_%d", feedID, hash&0x7FFFFFFF)
	}
	hash := 0
	for _, c := range item.Title {
		hash = ((hash << 5) - hash) + int(c)
	}
	return fmt.Sprintf("%s_title_%d", feedID, hash&0x7FFFFFFF)
}

// NewRSSFetcher creates an RSSFetcher for use with the shared scheduler.
// The msgBus parameter is accepted but not used in this version.
func NewRSSFetcher(dbPath string, msgBus interface{}) *RSSFetcher {
	return &RSSFetcher{}
}

// Fetch is the cron job that fetches all subscribed RSS feeds.
func (f *RSSFetcher) Fetch(ctx context.Context) {
	// No-op in this version — RSSFeedTool handles fetching on subscribe
	// and the shared scheduler calls this for periodic background fetches.
}
