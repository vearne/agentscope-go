---
name: Weather Query
description: Query current weather conditions for any city using shell commands.
---

# Weather Query Skill

Use the `execute_shell` tool to fetch weather data:

1. Run `curl -s "wttr.in/{city}?format=3"` for a brief summary
2. Run `curl -s "wttr.in/{city}"` for a detailed forecast

Replace `{city}` with the user's requested city name. If the city name contains spaces, use URL encoding (e.g., "New York" → "New%20York").
