import os
import time
import requests
import sys
import json

# Configuration
METABASE_URL = os.environ.get("METABASE_URL") or "http://metabase:3000"
ADMIN_EMAIL = os.environ.get("MB_ADMIN_EMAIL") or "admin@domain-os.local"
ADMIN_PASS = os.environ.get("MB_ADMIN_PASS") or "admin1234"
DB_NAME = os.environ.get("DB_NAME") or "domainos"
DB_HOST = os.environ.get("DB_HOST") or "db"
DB_PORT = os.environ.get("DB_PORT") or "5432"
DB_USER = os.environ.get("DB_USER") or "postgres"
DB_PASS = os.environ.get("DB_PASS") or "postgres"

def wait_for_metabase():
    """Polls Metabase until it is ready."""
    print(f"Waiting for Metabase at {METABASE_URL}...")
    for _ in range(60): # Wait up to 60 * 2 = 120 seconds
        try:
            res = requests.get(f"{METABASE_URL}/api/health", timeout=2)
            if res.status_code == 200:
                print("Metabase is up!")
                return True
        except requests.exceptions.ConnectionError:
            pass
        time.sleep(2)
    print("Metabase failed to start.")
    return False

def get_session_token():
    """Logs in and returns the session token."""
    payload = {
        "username": ADMIN_EMAIL,
        "password": ADMIN_PASS
    }
    res = requests.post(f"{METABASE_URL}/api/session", json=payload)
    if res.status_code == 200:
        return res.json()["id"]
    print(f"Failed to log in: {res.text}")
    return None

def initial_setup():
    """Performs the initial setup if not already done."""
    try:
        res = requests.get(f"{METABASE_URL}/api/session/properties")
        props = res.json()
    except Exception as e:
        print(f"Failed to get properties: {e}")
        return False
    
    if props.get("setup-token"):
        print("Metabase requires setup. Initializing...")
        setup_token = props["setup-token"]
        
        payload = {
            "token": setup_token,
            "user": {
                "first_name": "Admin",
                "last_name": "User",
                "email": ADMIN_EMAIL,
                "password": ADMIN_PASS
            },
            "prefs": {
                "site_name": "Domain OS Analytics",
                "allow_tracking": False
            }
        }
        res = requests.post(f"{METABASE_URL}/api/setup", json=payload)
        if res.status_code == 200:
            print("Setup complete.")
            return True
        else:
            print(f"Setup failed: {res.text}")
            return False
    else:
        print("Metabase already setup.")
        return True

def add_database(session_token):
    """Adds the Domain OS Postgres database if not present."""
    headers = {"X-Metabase-Session": session_token}
    
    # List existing databases
    try:
        res = requests.get(f"{METABASE_URL}/api/database", headers=headers)
        databases_res = res.json()
        databases = databases_res.get("data", [])
    except Exception as e:
        print(f"Failed to list databases: {e}")
        return

    # Debug response
    if not isinstance(databases, list):
        print(f"Unexpected response type for databases: {type(databases)}")
        print(f"Response content: {databases}")
        return
    
    for db in databases:
        # DB objects should be dicts
        if not isinstance(db, dict):
            print(f"Skipping invalid db entry: {db}")
            continue
            
        if db.get("name") == "Domain OS DB":
            print("Database 'Domain OS DB' already connected.")
            return

    print(f"Adding 'Domain OS DB' at {DB_HOST}:{DB_PORT}...")
    payload = {
        "name": "Domain OS DB",
        "engine": "postgres",
        "details": {
            "host": DB_HOST,
            "port": int(DB_PORT),
            "dbname": DB_NAME,
            "user": DB_USER,
            "password": DB_PASS,
            "ssl": False,
            "ssl-mode": "disable"
        }
    }
    
    res = requests.post(f"{METABASE_URL}/api/database", json=payload, headers=headers)
    if res.status_code == 200:
        print("Database added successfully.")
    else:
        print(f"Failed to add database: {res.text}")

def save_json(data, filename):
    filepath = os.path.join("/app/dashboards", filename)
    os.makedirs(os.path.dirname(filepath), exist_ok=True)
    with open(filepath, "w") as f:
        json.dump(data, f, indent=2)
    print(f"Saved {filename}")

def get_all_items(session_token, endpoint):
    headers = {"X-Metabase-Session": session_token}
    res = requests.get(f"{METABASE_URL}{endpoint}", headers=headers)
    if res.status_code == 200:
        return res.json()
    print(f"Failed to fetch {endpoint}: {res.text}")
    return []

def export_collections(session_token):
    print("Exporting collections...")
    collections = get_all_items(session_token, "/api/collection")
    save_json(collections, "collections.json")

def export_cards(session_token):
    print("Exporting cards (questions)...")
    cards = get_all_items(session_token, "/api/card")
    save_json(cards, "cards.json")

def export_dashboards(session_token):
    print("Exporting dashboards...")
    dashboards = get_all_items(session_token, "/api/dashboard")
    save_json(dashboards, "dashboards_list.json")
    
    for db in dashboards:
        db_id = db["id"]
        # Fetch full details including cards
        headers = {"X-Metabase-Session": session_token}
        res = requests.get(f"{METABASE_URL}/api/dashboard/{db_id}", headers=headers)
        if res.status_code == 200:
            db_detail = res.json()
            slug = db_detail.get("slug", f"dashboard_{db_id}")
            save_json(db_detail, f"dashboard_{db_id}_{slug}.json")
        else:
            print(f"Failed to fetch dashboard {db_id}")

def main():
    if not wait_for_metabase():
        sys.exit(1)
        
    initial_setup()
    
    token = get_session_token()
    if not token:
        print("Could not log in.")
        sys.exit(1)
        
    add_database(token)
    
    if len(sys.argv) > 1 and sys.argv[1] == "export":
        export_collections(token)
        export_cards(token)
        export_dashboards(token)

if __name__ == "__main__":
    main()
