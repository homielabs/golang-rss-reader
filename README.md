# golang-rss-client

## Configuration

Looks in the following locations for `golang-rss-client.yml`:

* `/etc/golang-rss-client/`
* `$HOME/golang-rss-client/`
* `.`

```yaml
# ansi colors. You can probably replace these with hex if you want (will be
# automatically converted to the closest color if required)
accent: "33"
textColor: "15"
backgroundColor: "233"
horzPadding: 2  # horizontal padding in header/footer components
vertPadding: 0  # vertical padding in header component
feedUrls: https://github.com/homielabs.atom
```

Or prefix environment variables with `GOLANGRSSCLIENT_`.
