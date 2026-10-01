package server

// Brand mark (favicon + logo): an EU-yellow shield — the compliance/egress-
// control story — carrying a single star for the EU, on navy. Single self-contained SVG, reused as the favicon and the nav logo,
// so there is one source of truth and no external asset to fetch.

import (
	"net/http"

	"github.com/magnusfroste/sluss/internal/buildinfo"
)

// brandFaviconSVG is the standalone favicon document: a navy rounded tile and
// an EU-yellow shield carrying a single navy star (EU palette, matching the
// product site www.sluss.eu). Deliberately ONE star, not the EU emblem's ring
// of twelve: the emblem must not suggest EU endorsement of a product.
const brandFaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img" aria-label="Sluss">
<rect width="32" height="32" rx="7" fill="#081328"/>
<path d="M16 3.6 L26 7.3 V15.5 C26 21.3 21.7 25.9 16 28.2 C10.3 25.9 6 21.3 6 15.5 V7.3 Z" fill="#fad100"/>
<path d="M16.00 8.70 L17.56 13.16 L22.28 13.26 L18.52 16.12 L19.88 20.64 L16.00 17.95 L12.12 20.64 L13.48 16.12 L9.72 13.26 L14.44 13.16 Z" fill="#081328"/>
</svg>`

// brandLogoSVG is the inline nav logo (18px), same mark without the tile so it
// sits on the panel background.
const brandLogoSVG = `<svg width="18" height="18" viewBox="0 0 32 32" aria-hidden="true" style="flex-shrink:0">
<path d="M16 3 L26 6.8 V15.5 C26 21.2 21.6 25.6 16 27.8 C10.4 25.6 6 21.2 6 15.5 V6.8 Z" fill="#fad100"/>
<path d="M16.00 8.70 L17.56 13.16 L22.28 13.26 L18.52 16.12 L19.88 20.64 L16.00 17.95 L12.12 20.64 L13.48 16.12 L9.72 13.26 L14.44 13.16 Z" fill="#081328"/>
</svg>`

// faviconLinks are the <link> tags every HTML page includes so the shield shows
// in the browser tab. SVG favicon with an emoji-free, asset-free footprint.
//
// The URL carries the build commit so a new build always busts the browser's
// cached icon (the favicon is served with a 24h max-age).
var faviconLinks = `<link rel="icon" type="image/svg+xml" href="/favicon.svg?v=` + buildinfo.Short() + `">` +
	`<link rel="apple-touch-icon" href="/favicon.svg?v=` + buildinfo.Short() + `">`

// FaviconHandler serves the shield SVG favicon (long-cached, immutable-ish).
func FaviconHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(brandFaviconSVG))
	}
}

// FaviconICOHandler redirects the browsers' default /favicon.ico probe to the
// SVG so we ship a single asset.
func FaviconICOHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/favicon.svg", http.StatusMovedPermanently)
	}
}
