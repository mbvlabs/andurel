package proxy

import (
	"bytes"
	"regexp"
	"strings"
)

var stylesheetHrefRegex = regexp.MustCompile(`(?is)<link\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>`)

const HotReloadScript = `<script>
(function() {
  var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  var wsUrl = protocol + '//' + window.location.host + '/__andurel/events';
  var reconnectDelay = 1000;
  var maxReconnectDelay = 5000;

  function reloadPage() {
    console.log('[run] Reloading page...');
    window.location.reload();
  }

  window.addEventListener('pageshow', function(event) {
    if (event.persisted) {
      reloadPage();
    }
  });

  function connect() {
    var ws = new WebSocket(wsUrl);

    ws.onopen = function() {
      console.log('[run] Connected to hot reload server');
      reconnectDelay = 1000;
    };

    ws.onmessage = function(event) {
      if (event.data === 'r') {
        reloadPage();
      }
    };

    ws.onclose = function() {
      console.log('[run] Connection closed, reconnecting in ' + reconnectDelay + 'ms');
      setTimeout(function() {
        reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
        connect();
      }, reconnectDelay);
    };

    ws.onerror = function(err) {
      console.log('[run] WebSocket error:', err);
      ws.close();
    };
  }

  connect();
})();
</script>`

func InjectScript(content []byte) []byte {
	script := []byte(HotReloadScript)

	headClose := bytes.Index(bytes.ToLower(content), []byte("</head>"))
	if headClose != -1 {
		result := make([]byte, len(content)+len(script))
		copy(result, content[:headClose])
		copy(result[headClose:], script)
		copy(result[headClose+len(script):], content[headClose:])
		return result
	}

	bodyClose := bytes.Index(bytes.ToLower(content), []byte("</body>"))
	if bodyClose != -1 {
		result := make([]byte, len(content)+len(script))
		copy(result, content[:bodyClose])
		copy(result[bodyClose:], script)
		copy(result[bodyClose+len(script):], content[bodyClose:])
		return result
	}

	return append(content, script...)
}

func RewriteStylesheetHrefs(content []byte) []byte {
	return stylesheetHrefRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		tag := string(match)
		lowerTag := strings.ToLower(tag)
		if !strings.Contains(lowerTag, "rel=") || !strings.Contains(lowerTag, "stylesheet") {
			return match
		}

		submatches := stylesheetHrefRegex.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		href := string(submatches[1])
		rewritten, ok := rewriteStylesheetHref(href)
		if !ok {
			return match
		}
		return []byte(strings.Replace(tag, href, rewritten, 1))
	})
}

func rewriteStylesheetHref(href string) (string, bool) {
	lowerHref := strings.ToLower(href)
	if strings.HasPrefix(lowerHref, "http://") || strings.HasPrefix(lowerHref, "https://") || strings.HasPrefix(lowerHref, "//") || strings.HasPrefix(lowerHref, "data:") {
		return "", false
	}

	pathPart := href
	suffix := ""
	if idx := strings.IndexAny(href, "?#"); idx >= 0 {
		pathPart = href[:idx]
		suffix = href[idx:]
	}

	if pathPart == "" {
		return "", false
	}

	normalized := strings.TrimPrefix(pathPart, "./")
	normalized = normalizeAssetPathForRewrite(normalized)
	if strings.HasPrefix(normalized, "/assets/dist/") || strings.HasPrefix(normalized, "assets/dist/") {
		return "", false
	}
	if strings.HasPrefix(normalized, "/__andurel/assets/") {
		return "", false
	}

	if strings.HasPrefix(normalized, "/assets/") {
		return "/__andurel" + normalized + suffix, true
	}
	if strings.HasPrefix(normalized, "assets/") {
		return "/__andurel/" + normalized + suffix, true
	}

	return "", false
}

func normalizeAssetPathForRewrite(path string) string {
	if strings.HasPrefix(path, "/assets/") {
		return stripCacheBusterSegment(path, "/assets/")
	}
	if strings.HasPrefix(path, "assets/") {
		return stripCacheBusterSegment(path, "assets/")
	}
	return path
}

func stripCacheBusterSegment(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 3 {
		return path
	}

	for i := 1; i < len(parts)-1; i++ {
		if isCacheBusterSegment(parts[i]) {
			trimmed := append([]string{}, parts[:i]...)
			trimmed = append(trimmed, parts[i+1:]...)
			if strings.HasSuffix(path, "/") {
				return prefix + strings.Join(trimmed, "/") + "/"
			}
			return prefix + strings.Join(trimmed, "/")
		}
	}
	return path
}

func IsHTMLResponse(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html")
}
