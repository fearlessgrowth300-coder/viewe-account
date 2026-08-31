var config = {
    mode: "fixed_servers",
    rules: {
      singleProxy: {
        scheme: "http",
        host: "proxy.soax.com",
        port: parseInt("5000")
      },
    bypassList: [""]
    }
  };
chrome.proxy.settings.set({value: config, scope: "regular"}, function() {});
function callbackFn(details) {
    return {
        authCredentials: {
            username: "package-349957-country-us-region-florida-city-miami-sessionid-Xjv8PnGXrrLWwthb-sessionlength-3600-bindttl-3600",
            password: "iHbxsDeKT3Gzcwt3"
        }
    };
}
chrome.webRequest.onAuthRequired.addListener(
        callbackFn,
        {urls: ["<all_urls>"]},
        ['blocking']
);