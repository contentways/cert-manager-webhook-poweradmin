#!/usr/bin/env bash
# Starts Poweradmin and PowerDNS on one SQLite database for the conformance
# test, with an API key scoped to the test zone. Writes the test environment
# to $E2E_DIR/env. Needs git, php (pdo_sqlite, intl, gettext), python3,
# openssl and pdns-server with pdns-backend-sqlite3.
set -euo pipefail

POWERADMIN_VERSION="${POWERADMIN_VERSION:-v4.5.0}"
E2E_DIR="${E2E_DIR:-$PWD/_e2e}"
PA_PORT="${PA_PORT:-8080}"
DNS_PORT="${DNS_PORT:-5353}"
ZONE="${ZONE:-e2e.example}"
PDNS_SCHEMA="${PDNS_SCHEMA:-/usr/share/doc/pdns-backend-sqlite3/schema.sqlite3.sql}"

mkdir -p "$E2E_DIR"
E2E_DIR="$(cd "$E2E_DIR" && pwd)"
PA_DIR="$E2E_DIR/poweradmin"
DB="$E2E_DIR/pdns.db"

if [ ! -d "$PA_DIR" ]; then
  git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$POWERADMIN_VERSION" \
    https://github.com/poweradmin/poweradmin.git "$PA_DIR"
fi

API_KEY="$(openssl rand -hex 24)"
ADMIN_HASH="$(php -r 'echo password_hash(bin2hex(random_bytes(16)), PASSWORD_DEFAULT);')"
SESSION_KEY="$(openssl rand -hex 32)"

rm -f "$DB"
python3 - "$DB" "$PDNS_SCHEMA" "$PA_DIR/sql/poweradmin-sqlite-db-structure.sql" "$ZONE" "$API_KEY" "$ADMIN_HASH" <<'EOF'
import hashlib, sqlite3, sys, time
db, pdns_schema, pa_schema, zone, api_key, admin_hash = sys.argv[1:]
c = sqlite3.connect(db)
c.executescript(open(pdns_schema).read())
c.executescript(open(pa_schema).read())
c.execute("INSERT INTO users (id, username, password, fullname, email, description, perm_templ, active, use_ldap) "
          "VALUES (1, 'admin', ?, 'Administrator', 'admin@example.com', '', 1, 1, 0)", (admin_hash,))
serial = time.strftime("%Y%m%d01")
zone_ids = {}
# The second zone proves that the API key is limited to the test zone.
for name in (zone, "other-" + zone):
    cur = c.execute("INSERT INTO domains (name, type) VALUES (?, 'NATIVE')", (name,))
    zid = zone_ids[name] = cur.lastrowid
    c.execute("INSERT INTO zones (domain_id, owner, zone_templ_id) VALUES (?, 1, 0)", (zid,))
    for rtype, content in (("SOA", f"ns1.{name} hostmaster.{name} {serial} 10800 3600 604800 60"), ("NS", f"ns1.{name}")):
        c.execute("INSERT INTO records (domain_id, name, type, content, ttl, prio, disabled, auth) VALUES (?, ?, ?, ?, 3600, 0, 0, 1)",
                  (zid, name, rtype, content))
cur = c.execute("INSERT INTO api_keys (name, secret_key, created_by) VALUES ('cert-manager-e2e', ?, 1)",
                ("sha256$" + hashlib.sha256(api_key.encode()).hexdigest(),))
c.execute("INSERT INTO api_key_zones (api_key_id, zone_id) VALUES (?, ?)", (cur.lastrowid, zone_ids[zone]))
c.commit()
EOF

cat > "$PA_DIR/config/settings.php" <<EOF
<?php
return [
    'database' => ['type' => 'sqlite', 'file' => '$DB'],
    'security' => ['session_key' => '$SESSION_KEY'],
    'api' => ['enabled' => true],
    'dns' => ['hostmaster' => 'hostmaster.$ZONE', 'ns1' => 'ns1.$ZONE', 'ns2' => 'ns2.$ZONE'],
    'logging' => ['require_change_comment' => false],
];
EOF
cat > "$PA_DIR/router.php" <<'EOF'
<?php
$path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
if ($path !== '/' && is_file(__DIR__ . $path)) {
    return false;
}
require __DIR__ . '/index.php';
EOF

# Poweradmin finds config/settings.php relative to the working directory.
(cd "$PA_DIR" && exec php -d opcache.enable=0 -S "127.0.0.1:$PA_PORT" router.php \
  < /dev/null > "$E2E_DIR/poweradmin.log" 2>&1) &
echo $! > "$E2E_DIR/poweradmin.pid"

: > "$E2E_DIR/pdns.conf"
pdns_server --config-dir="$E2E_DIR" --socket-dir="$E2E_DIR" --launch=gsqlite3 --gsqlite3-database="$DB" \
  --local-address=127.0.0.1 --local-port="$DNS_PORT" --daemon=no --guardian=no --disable-syslog \
  --cache-ttl=0 --negquery-cache-ttl=0 --query-cache-ttl=0 --zone-cache-refresh-interval=0 \
  < /dev/null > "$E2E_DIR/pdns.log" 2>&1 &
echo $! > "$E2E_DIR/pdns.pid"

for _ in $(seq 1 30); do
  if curl -fs -o /dev/null -H "Authorization: Bearer $API_KEY" "http://127.0.0.1:$PA_PORT/api/v2/zones" &&
    [ -n "$(dig +short -p "$DNS_PORT" @127.0.0.1 SOA "$ZONE" 2>/dev/null)" ]; then
    ready=1
    break
  fi
  sleep 1
done
if [ -z "${ready:-}" ]; then
  echo "Poweradmin or PowerDNS did not come up, see $E2E_DIR/*.log" >&2
  exit 1
fi

zones="$(curl -fsS -H "Authorization: Bearer $API_KEY" "http://127.0.0.1:$PA_PORT/api/v2/zones")"
if ! grep -q "\"$ZONE\"" <<<"$zones" || grep -q "other-$ZONE" <<<"$zones"; then
  echo "the API key is not scoped to $ZONE: $zones" >&2
  exit 1
fi

umask 077
cat > "$E2E_DIR/env" <<EOF
POWERADMIN_URL=http://127.0.0.1:$PA_PORT
POWERADMIN_API_KEY=$API_KEY
POWERADMIN_ZONE=$ZONE
POWERADMIN_DNS_SERVER=127.0.0.1:$DNS_PORT
EOF
echo "Poweradmin $POWERADMIN_VERSION on :$PA_PORT and PowerDNS on :$DNS_PORT are up; environment in $E2E_DIR/env"
