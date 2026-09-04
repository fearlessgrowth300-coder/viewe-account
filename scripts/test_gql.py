import requests

gql_url = 'https://gql.twitch.tv/gql'
headers = {
    'Client-Id': 'kimne78kx3ncx6brgo4mv6wki5h1ko',
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
    'Origin': 'https://www.twitch.tv',
    'Referer': 'https://www.twitch.tv/agenciapx'
}

# Test 1: playerType: site
payload1 = {
    'operationName': 'PlaybackAccessToken',
    'variables': {
        'isLive': True,
        'login': 'agenciapx',
        'isVod': False,
        'vodID': '',
        'playerType': 'site'
    },
    'extensions': {
        'persistedQuery': {
            'version': 1,
            'sha256Hash': '0828119ded1c13477966434e15800f40b819e9d8481abbfb50d73474b7d4d70c'
        }
    }
}
r1 = requests.post(gql_url, json=payload1, headers=headers)
print('Test 1 (site):', r1.status_code, r1.text)

# Test 2: playerType: embed
payload2 = {
    'operationName': 'PlaybackAccessToken',
    'variables': {
        'isLive': True,
        'login': 'agenciapx',
        'isVod': False,
        'vodID': '',
        'playerType': 'embed'
    },
    'extensions': {
        'persistedQuery': {
            'version': 1,
            'sha256Hash': '0828119ded1c13477966434e15800f40b819e9d8481abbfb50d73474b7d4d70c'
        }
    }
}
r2 = requests.post(gql_url, json=payload2, headers=headers)
print('Test 2 (embed):', r2.status_code, r2.text)

# Test 3: playerType: frontpage
payload3 = {
    'operationName': 'PlaybackAccessToken',
    'variables': {
        'isLive': True,
        'login': 'agenciapx',
        'isVod': False,
        'vodID': '',
        'playerType': 'frontpage'
    },
    'extensions': {
        'persistedQuery': {
            'version': 1,
            'sha256Hash': '0828119ded1c13477966434e15800f40b819e9d8481abbfb50d73474b7d4d70c'
        }
    }
}
r3 = requests.post(gql_url, json=payload3, headers=headers)
print('Test 3 (frontpage):', r3.status_code, r3.text)
