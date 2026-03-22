# Recipes

These recipes focus on the two primary `dimlox` personas: the data mover and
the pipeline tester.

## Download -> inspect -> split a cloud file

Use this when you need one repeatable path from remote object to local shards.

```bash
dimlox doctor "gs://example-bucket/data/orders.psv.gz"
dimlox get --verify "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
dimlox inspect --detect "/tmp/orders.psv.gz"
dimlox inspect --wc "/tmp/orders.psv.gz"
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

Why this recipe works:

- `doctor` catches auth or path issues early
- `inspect` confirms shape before you commit to a long split
- `split --manifest` leaves an audit trail for every shard

## CI-friendly split with manifest verification

Use this when QA or pipeline automation needs deterministic shard metadata.

```bash
dimlox split --dry-run --rows 1000000 --header --out-dir "/tmp/ci-shards" "/tmp/orders.psv"
dimlox split --rows 1000000 --header --manifest --out-dir "/tmp/ci-shards" "/tmp/orders.psv"
python3 - <<'PY'
import json
from pathlib import Path

manifest = Path("/tmp/ci-shards/orders_manifest.jsonl")
entries = [json.loads(line) for line in manifest.read_text().splitlines() if line.strip()]
assert entries, "manifest is empty"
assert all(entry.get("shard_path") for entry in entries), "missing shard_path"
print(f"verified {len(entries)} manifest entries")
PY
```

Why this recipe works:

- `--dry-run` lets CI fail early on bad assumptions without writing shards
- the manifest gives automation a stable file to inspect after the split

## Cross-provider migration with a local landing area

Use this when the file needs to move clouds before downstream processing.

```bash
dimlox cp --verify --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
dimlox doctor "azblob://exampleaccount/example-container/data/orders.psv"
```

Why this recipe works:

- `cp` keeps the two-leg transfer explicit
- `--landing` makes disk usage predictable
- the follow-up `doctor` confirms the destination object is visible

## Handle compressed cloud data safely

Use this when the source is a remote gzip file and you need more than a quick head sample.

```bash
dimlox inspect --head 5 "gs://example-bucket/data/orders.psv.gz"
dimlox get "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
dimlox inspect --tail 5 "/tmp/orders.psv.gz"
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

Why this recipe works:

- `inspect --head` is cheap on the remote gzip object
- download-first avoids expensive remote forward-stream tail or mid operations
- the local gzip file still splits in stream mode without a full memory load

## Related docs

- [`docs/adoption/quickstart.md`](quickstart.md)
- [`docs/adoption/positioning.md`](positioning.md)
- [`docs/usage/get.md`](../usage/get.md)
- [`docs/usage/inspect.md`](../usage/inspect.md)
- [`docs/usage/split.md`](../usage/split.md)
