Output demo.gif

Require grafana-tui

Set Shell zsh
Set FontSize 18
Set Width 1200
Set Height 800
Set Padding 20
Set WindowBar Colorful
Set Theme "Catppuccin Frappe"
Set Margin 20
Set MarginFill "#1E1E2E"
Set BorderRadius 8
Set TypingSpeed 50ms
Set CursorBlink false

# Launch grafana-tui
Type "grafana-tui --url https://play.grafana.org"
Enter
Sleep 3s

# Filter dashboards
Type "/"
Sleep 500ms
Type "examples bar"
Enter
Sleep 2s

# Select the first dashboard
Enter
Sleep 3s

# Panel 1 is already shown, wait then scroll
Down
Sleep 3s

# Panel 3
Down
Sleep 3s

# Panel 4
Down
Sleep 3s

# Panel 5
Down
Sleep 3s
