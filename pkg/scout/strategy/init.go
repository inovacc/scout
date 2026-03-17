package strategy

// Template is a minimal strategy YAML template.
const Template = `name: my-strategy
version: "1.0"

browser:
  type: chrome
  stealth: true
  headless: true
  # proxy: socks5://proxy:1080
  # user_agent: "custom..."
  # window_size: [1920, 1080]

# auth:
#   provider: slack
#   session: ~/.scout/sessions/slack.json
#   passphrase: ${SCOUT_PASSPHRASE}
#   capture_on_close: true
#   timeout: 5m

steps:
  - name: scrape-data
    mode: slack
    targets:
      - general
    limit: 100
    timeout: 10m
    # when:
    #   has_auth: true

  # - name: scrape-reviews
  #   mode: gmaps
  #   targets:
  #     - "pizza near me"
  #   limit: 50

output:
  report: false
  sinks:
    - type: json-file
      path: ./results/output.json
    # - type: ndjson
    #   path: ./results/output.ndjson
    # - type: csv
    #   path: ./results/output.csv
`
