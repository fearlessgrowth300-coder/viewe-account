import re
import requests
import os

def harvest_public_proxies(output_file: str = "proxies.txt"):
    """
    Harvests public HTTP/HTTPS proxies from open-source endpoints,
    deduplicates them, and writes them to a shared text file.
    """
    print("[Python Scraper] Initiating mass proxy harvest...")

    sources = [
        "https://api.proxyscrape.com/v2/?request=getproxies&protocol=http&timeout=5000&country=all&ssl=all&anonymity=all",
        "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt",
        "https://raw.githubusercontent.com/monosans/proxy-list/main/proxies/http.txt",
        "https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt",
    ]

    raw_proxies = set()
    ip_port_pattern = re.compile(r'\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]{1,5}\b')
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
    }

    for url in sources:
        domain = url.split("/")[2]
        try:
            print(f"[Scraper] Fetching proxies from: {domain}")
            response = requests.get(url, headers=headers, timeout=10)
            if response.status_code == 200:
                matches = ip_port_pattern.findall(response.text)
                print(f"[Scraper] Found {len(matches)} entries from {domain}")
                for proxy in matches:
                    raw_proxies.add(f"http://{proxy}")
            else:
                print(f"[Warning] {domain} returned HTTP {response.status_code}")
        except Exception as e:
            print(f"[Warning] Error fetching from {domain}: {e}")

    try:
        with open(output_file, "w", encoding="utf-8") as f:
            for proxy in sorted(raw_proxies):
                f.write(proxy + "\n")
        print(f"[Success] Saved {len(raw_proxies)} unique proxies to {os.path.abspath(output_file)}")
    except IOError as e:
        print(f"[Error] Failed to write to {output_file}: {e}")

if __name__ == "__main__":
    target_path = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "proxies.txt")
    harvest_public_proxies(target_path)
