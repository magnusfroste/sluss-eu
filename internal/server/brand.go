package server

// Brand mark (favicon + logo): a shield — the compliance/egress-control story —
// with an inner routing split (a stem branching to two nodes) = "protected
// routing". Single self-contained SVG, reused as the favicon and the nav logo,
// so there is one source of truth and no external asset to fetch.

import "net/http"

// brandFaviconSVG is the standalone favicon document: a navy rounded tile, an
// EU-yellow shield, and a routing split cut into it (EU palette, matching the
// product site www.sluss.eu).
const brandFaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" role="img" aria-label="Sluss">
<rect width="32" height="32" rx="7" fill="#081328"/>
<path d="M16 5 L25 8.4 V15.5 C25 20.6 21 24.6 16 26.6 C11 24.6 7 20.6 7 15.5 V8.4 Z" fill="#fad100"/>
<g fill="none" stroke="#081328" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
  <path d="M16 20.5 V16 M16 16 C16 13.6 13.4 13.4 12.4 11.6 M16 16 C16 13.6 18.6 13.4 19.6 11.6"/>
</g>
<g fill="#081328">
  <circle cx="16" cy="21" r="1.5"/><circle cx="12" cy="11" r="1.5"/><circle cx="20" cy="11" r="1.5"/>
</g>
</svg>`

// brandLogoSVG is the inline nav logo (18px), same mark without the tile so it
// sits on the panel background.
const brandLogoSVG = `<svg width="18" height="18" viewBox="0 0 32 32" aria-hidden="true" style="flex-shrink:0">
<path d="M16 3 L26 6.8 V15.5 C26 21.2 21.6 25.6 16 27.8 C10.4 25.6 6 21.2 6 15.5 V6.8 Z" fill="#fad100"/>
<g fill="none" stroke="#081328" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
  <path d="M16 21 V16 M16 16 C16 13.4 13.2 13.2 12.2 11.2 M16 16 C16 13.4 18.8 13.2 19.8 11.2"/>
</g>
<g fill="#081328">
  <circle cx="16" cy="21.4" r="1.6"/><circle cx="12" cy="10.6" r="1.6"/><circle cx="20" cy="10.6" r="1.6"/>
</g>
</svg>`

// faviconLinks are the <link> tags every HTML page includes so the shield shows
// in the browser tab. SVG favicon with an emoji-free, asset-free footprint.
const faviconLinks = `<link rel="icon" type="image/svg+xml" href="/favicon.svg">` +
	`<link rel="apple-touch-icon" href="/favicon.svg">`

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
