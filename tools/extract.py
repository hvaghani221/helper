import sys

import pandas as pd

df = pd.read_csv(sys.argv[1])[['machine_friendly_name', 'OpenAPI spec']]
new_df = df.rename(
    columns={'machine_friendly_name': 'name', 'OpenAPI spec': 'spec_url'})
json_result = new_df.to_json(orient='records')

with open("api.json", "w") as f:
    f.write(json_result)
