import os
import json

# This is all the liar's code (ChatGPT)

# Load the options passed from Home Assistant
options_path = '/data/options.json'
with open(options_path) as f:
    options = json.load(f)

# Build disallowed prefixes YAML format
disallowed_prefixes = options.get('duc_disallowed_prefixes', [])
disallowed_prefixes_yaml = '\n'.join([f'    - {prefix}' for prefix in disallowed_prefixes])

# Generate config.yaml content
config_yaml = f"""
mqtt:
  url: {options['mqtt_url']}
  uniqueId: {options['mqtt_unique_id']}
  topicPrefix: {options['mqtt_topic_prefix']}
duc:
  url: {options['duc_url']}
  disallowedPrefixes:
{disallowed_prefixes_yaml}
intervalSeconds: {options['interval_seconds']}
"""

# Write config.yaml to /config/duc2mqtt/config.yaml (or a path inside the container)
config_path = '/config/duc2mqtt/config.yaml'
os.makedirs(os.path.dirname(config_path), exist_ok=True)
with open(config_path, 'w') as f:
    f.write(config_yaml)

print(f"Configuration written to {config_path}")
