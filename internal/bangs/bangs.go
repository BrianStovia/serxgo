package bangs

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"searxgo/internal/models"
)

type ParsedBangResult struct {
	CleanQuery        string
	Category          models.Category
	Engines           []string
	ExcludedEngines   []string
	Language          string
	Country           string
	Region            string
	TimeoutLimit      time.Duration
	DirectRedirectURL string
	SiteFilter        string
	ExcludedSites     []string
	Filetype          string
	Intitle           string
	Inurl             string
	ExactPhrase       string
	DateAfter         *time.Time
	DateBefore        *time.Time
	MustTerms         []string
	MustNotTerms      []string
	OrTerms           [][]string
	DeepSearch        bool
	CrossCategory     bool
}

var categoryBangs = map[string]models.Category{
	"!general": models.CategoryGeneral,
	"!gen": models.CategoryGeneral,
	"!images": models.CategoryImages,
	"!image": models.CategoryImages,
	"!img": models.CategoryImages,
	"!videos": models.CategoryVideos,
	"!video": models.CategoryVideos,
	"!vid": models.CategoryVideos,
	"!news": models.CategoryNews,
	"!it": models.CategoryIT,
	"!code": models.CategoryIT,
	"!dev": models.CategoryIT,
	"!science": models.CategoryScience,
	"!sci": models.CategoryScience,
	"!social": models.CategorySocial,
	"!files": models.CategoryFiles,
	"!file": models.CategoryFiles,
	"!tor": models.CategoryFiles,
	"!torrent": models.CategoryFiles,
	"!music": models.CategoryMusic,
	"!audio": models.CategoryMusic,
	"!maps": models.CategoryMaps,
	"!map": models.CategoryMaps,
	"!other": models.CategoryOther,
}

var engineBangs = map[string]string{
	"!1337x": "1337x",
	"!1x": "1x",
	"!360search": "360search",
	"!360search_videos": "360search_videos",
	"!360so": "360search",
	"!360sov": "360search_videos",
	"!500": "500px",
	"!500px": "500px",
	"!9g": "9gag",
	"!9gag": "9gag",
	"!aa": "annas_archive",
	"!abc": "abcnyheter",
	"!abcnyheter": "abcnyheter",
	"!acf": "acfun",
	"!acfun": "acfun",
	"!adobe_stock": "adobe_stock",
	"!adobe_stock_audio": "adobe_stock_audio",
	"!adobe_stock_video": "adobe_stock_video",
	"!al": "arch_linux_wiki",
	"!alp": "alpine_linux_packages",
	"!alpine_linux_packages": "alpine_linux_packages",
	"!anaconda": "anaconda",
	"!annas_archive": "annas_archive",
	"!ans": "ansa",
	"!ansa": "ansa",
	"!apk_mirror": "apk_mirror",
	"!apkm": "apk_mirror",
	"!apm": "apple_maps",
	"!apple_app_store": "apple_app_store",
	"!apple_maps": "apple_maps",
	"!aps": "apple_app_store",
	"!arc": "artic",
	"!arch_linux_wiki": "arch_linux_wiki",
	"!artic": "artic",
	"!artstation": "artstation",
	"!arx": "arxiv",
	"!arxiv": "arxiv",
	"!as": "artstation",
	"!asa": "adobe_stock_audio",
	"!asi": "adobe_stock",
	"!askubuntu": "askubuntu",
	"!asv": "adobe_stock_video",
	"!ayo": "ayo",
	"!baidu": "baidu",
	"!baidu_images": "baidu_images",
	"!baidu_kaifa": "baidu_kaifa",
	"!bandcamp": "bandcamp",
	"!bb": "bitbucket",
	"!bc": "bandcamp",
	"!bd": "baidu",
	"!bdi": "baidu_images",
	"!bdk": "baidu_kaifa",
	"!bi": "bing",
	"!bii": "bing_images",
	"!bil": "bilibili",
	"!bilibili": "bilibili",
	"!bin": "bing_news",
	"!bing": "bing",
	"!bing_images": "bing_images",
	"!bing_news": "bing_news",
	"!bing_videos": "bing_videos",
	"!bit": "bitchute",
	"!bitbucket": "bitbucket",
	"!bitchute": "bitchute",
	"!biv": "bing_videos",
	"!boa": "boardreader",
	"!boardreader": "boardreader",
	"!bpb": "bpb",
	"!br": "brave",
	"!brave": "brave",
	"!bt": "btdigg",
	"!bt4g": "bt4g",
	"!btdigg": "btdigg",
	"!ca": "cara",
	"!cachy_os_packages": "cachy_os_packages",
	"!cara": "cara",
	"!cb": "codeberg",
	"!cc": "currency",
	"!chef": "chefkoch",
	"!chefkoch": "chefkoch",
	"!codeberg": "codeberg",
	"!conda": "anaconda",
	"!cos": "cachy_os_packages",
	"!cpan": "metacpan",
	"!cr": "crossref",
	"!crossref": "crossref",
	"!crowdview": "crowdview",
	"!currency": "currency",
	"!cv": "crowdview",
	"!dailymotion": "dailymotion",
	"!dc": "dictzone",
	"!ddd": "ddg_definitions",
	"!ddg": "duckduckgo",
	"!ddg_definitions": "ddg_definitions",
	"!ddgw": "duckduckgo_web",
	"!ddi": "duckduckgo_images",
	"!ddn": "duckduckgo_news",
	"!ddv": "duckduckgo_videos",
	"!ddw": "duckduckgo_weather",
	"!deezer": "deezer",
	"!destat": "destatis",
	"!destatis": "destatis",
	"!devicons": "devicons",
	"!dh": "docker_hub",
	"!di": "devicons",
	"!dictzone": "dictzone",
	"!dm": "dailymotion",
	"!eco": "ecosia",
	"!ecoi": "ecosia_images",
	"!ecosia": "ecosia",
	"!docker_hub": "docker_hub",
	"!dog": "dogpile",
	"!dogi": "dogpile_images",
	"!dogpile": "dogpile",
	"!dogpile_images": "dogpile_images",
	"!du": "duden",
	"!duckduckgo": "duckduckgo",
	"!duckduckgo_images": "duckduckgo_images",
	"!duckduckgo_news": "duckduckgo_news",
	"!duckduckgo_videos": "duckduckgo_videos",
	"!duckduckgo_weather": "duckduckgo_weather",
	"!duckduckgo_web": "duckduckgo_web",
	"!duden": "duden",
	"!dz": "deezer",
	"!em": "emojipedia",
	"!emojipedia": "emojipedia",
	"!encyclosearch": "encyclosearch",
	"!erowid": "erowid",
	"!es": "encyclosearch",
	"!et": "etymonline",
	"!etymonline": "etymonline",
	"!ew": "erowid",
	"!fa": "fastbot",
	"!fastbot": "fastbot",
	"!fd": "fdroid",
	"!fdroid": "fdroid",
	"!fif": "findfiles",
	"!fifi": "findfiles_images",
	"!fifm": "findfiles_music",
	"!fifv": "findfiles_videos",
	"!findfiles": "findfiles",
	"!findfiles_images": "findfiles_images",
	"!findfiles_music": "findfiles_music",
	"!findfiles_videos": "findfiles_videos",
	"!findthatmeme": "findthatmeme",
	"!fire": "fireball",
	"!fireball": "fireball",
	"!fireball_news": "fireball_news",
	"!fireball_videos": "fireball_videos",
	"!firen": "fireball_news",
	"!firev": "fireball_videos",
	"!fl": "flickr",
	"!flaticon": "flaticon",
	"!fli": "flaticon",
	"!flickr": "flickr",
	"!free_software_directory": "free_software_directory",
	"!frinkiac": "frinkiac",
	"!frk": "frinkiac",
	"!fsd": "free_software_directory",
	"!ftm": "findthatmeme",
	"!fy": "fyyd",
	"!fynd": "fynd",
	"!fyyd": "fyyd",
	"!gab": "gabanza",
	"!gabanza": "gabanza",
	"!ge": "gentoo",
	"!geiz": "geizhals",
	"!geizhals": "geizhals",
	"!gen": "genius",
	"!genius": "genius",
	"!gentoo": "gentoo",
	"!gh": "github",
	"!gif": "giphy",
	"!giphy": "giphy",
	"!github": "github",
	"!gitlab": "gitlab",
	"!gl": "gitlab",
	"!gmx": "gmx",
	"!go": "google",
	"!goc": "google_cse",
	"!goci": "google_cse_images",
	"!goi": "google_images",
	"!gon": "google_news",
	"!good": "goodreads",
	"!goodreads": "goodreads",
	"!google": "google",
	"!google_cse": "google_cse",
	"!google_cse_images": "google_cse_images",
	"!google_images": "google_images",
	"!google_news": "google_news",
	"!google_play_apps": "google_play_apps",
	"!google_play_movies": "google_play_movies",
	"!google_scholar": "google_scholar",
	"!google_videos": "google_videos",
	"!gos": "google_scholar",
	"!gov": "google_videos",
	"!gpa": "google_play_apps",
	"!gpm": "google_play_movies",
	"!habr": "habrahabr",
	"!habrahabr": "habrahabr",
	"!hackernews": "hackernews",
	"!hex": "hex",
	"!hf": "huggingface",
	"!hfd": "huggingface_datasets",
	"!hfs": "huggingface_spaces",
	"!hn": "hackernews",
	"!ho": "hoogle",
	"!hoogle": "hoogle",
	"!huggingface": "huggingface",
	"!huggingface_datasets": "huggingface_datasets",
	"!huggingface_spaces": "huggingface_spaces",
	"!il_post": "il_post",
	"!imdb": "imdb",
	"!img": "imgur",
	"!imgur": "imgur",
	"!in": "ina",
	"!ina": "ina",
	"!ip": "ipernity",
	"!ipernity": "ipernity",
	"!iq": "iqiyi",
	"!iqiyi": "iqiyi",
	"!iseek": "iseek",
	"!isk": "iseek",
	"!jisho": "jisho",
	"!js": "jisho",
	"!kc": "kickass",
	"!kickass": "kickass",
	"!leco": "lemmy_communities",
	"!lecom": "lemmy_comments",
	"!lemmy_comments": "lemmy_comments",
	"!lemmy_communities": "lemmy_communities",
	"!lemmy_posts": "lemmy_posts",
	"!lemmy_users": "lemmy_users",
	"!lepo": "lemmy_posts",
	"!leus": "lemmy_users",
	"!lg": "library_genesis",
	"!library_genesis": "library_genesis",
	"!library_of_congress": "library_of_congress",
	"!lingva": "lingva",
	"!loc": "library_of_congress",
	"!luc": "lucide",
	"!lucide": "lucide",
	"!lux": "luxxle",
	"!luxxle": "luxxle",
	"!lv": "lingva",
	"!mag": "magnific",
	"!magnific": "magnific",
	"!mah": "mastodon_hashtags",
	"!man": "mankier",
	"!mankier": "mankier",
	"!mar": "marginalia",
	"!marginalia": "marginalia",
	"!mastodon_hashtags": "mastodon_hashtags",
	"!mastodon_users": "mastodon_users",
	"!material_icons": "material_icons",
	"!mau": "mastodon_users",
	"!mc": "mixcloud",
	"!mcw": "minecraft_wiki",
	"!mdn": "mdn",
	"!mediathekviewweb": "mediathekviewweb",
	"!metacpan": "metacpan",
	"!mi": "material_icons",
	"!microsoft_learn": "microsoft_learn",
	"!minecraft_wiki": "minecraft_wiki",
	"!mixcloud": "mixcloud",
	"!mjk": "mojeek",
	"!mjkimg": "mojeek_images",
	"!mjknews": "mojeek_news",
	"!mo": "mojeek",
	"!moj": "mojeek",
	"!mojeek": "mojeek",
	"!mojeek_images": "mojeek_images",
	"!mojeek_news": "mojeek_news",
	"!moviepilot": "moviepilot",
	"!mozhi": "mozhi",
	"!mp": "moviepilot",
	"!msl": "microsoft_learn",
	"!mvw": "mediathekviewweb",
	"!mwm": "mwmbl",
	"!mwmbl": "mwmbl",
	"!mymemory_translated": "mymemory_translated",
	"!mz": "mozhi",
	"!national_vulnerability_database": "national_vulnerability_database",
	"!naver": "naver",
	"!naver_images": "naver_images",
	"!naver_news": "naver_news",
	"!naver_videos": "naver_videos",
	"!nico": "niconico",
	"!niconico": "niconico",
	"!nixos_wiki": "nixos_wiki",
	"!nixw": "nixos_wiki",
	"!npm": "npm",
	"!nt": "nyaa",
	"!nvd": "national_vulnerability_database",
	"!nvr": "naver",
	"!nvri": "naver_images",
	"!nvrn": "naver_news",
	"!nvrv": "naver_videos",
	"!nyaa": "nyaa",
	"!oa": "openalex",
	"!oad": "openairedatasets",
	"!oap": "openairepublications",
	"!od": "odysee",
	"!odysee": "odysee",
	"!ol": "openlibrary",
	"!ollama": "ollama",
	"!om": "openmeteo",
	"!openairedatasets": "openairedatasets",
	"!openairepublications": "openairepublications",
	"!openalex": "openalex",
	"!openlibrary": "openlibrary",
	"!openmeteo": "openmeteo",
	"!openrepos": "openrepos",
	"!openstreetmap": "openstreetmap",
	"!openverse": "openverse",
	"!opv": "openverse",
	"!or": "openrepos",
	"!osm": "openstreetmap",
	"!pack": "packagist",
	"!packagist": "packagist",
	"!pdb": "pdbe",
	"!pdbe": "pdbe",
	"!pdia": "public_domain_image_archive",
	"!pe": "pexels",
	"!peertube": "peertube",
	"!pexels": "pexels",
	"!ph": "photon",
	"!photon": "photon",
	"!picjumbo": "picjumbo",
	"!pin": "pinterest",
	"!pinterest": "pinterest",
	"!piratebay": "piratebay",
	"!pixabay_images": "pixabay_images",
	"!pixabay_videos": "pixabay_videos",
	"!pixi": "pixabay_images",
	"!pixv": "pixabay_videos",
	"!pj": "picjumbo",
	"!poc": "podchaser",
	"!podchaser": "podchaser",
	"!privacywall": "privacywall",
	"!privacywall_images": "privacywall_images",
	"!privacywall_videos": "privacywall_videos",
	"!pst": "il_post",
	"!ptb": "peertube",
	"!pub": "pubmed",
	"!public_domain_image_archive": "public_domain_image_archive",
	"!pubmed": "pubmed",
	"!pw": "privacywall",
	"!pwi": "privacywall_images",
	"!pwv": "privacywall_videos",
	"!pypi": "pypi",
	"!qk": "quark",
	"!qki": "quark_images",
	"!quark": "quark",
	"!quark_images": "quark_images",
	"!qw": "qwant",
	"!qwant": "qwant",
	"!qwant_images": "qwant_images",
	"!qwant_news": "qwant_news",
	"!qwant_videos": "qwant_videos",
	"!qwi": "qwant_images",
	"!qwn": "qwant_news",
	"!qwv": "qwant_videos",
	"!radio_browser": "radio_browser",
	"!rb": "radio_browser",
	"!rbg": "rubygems",
	"!reh": "resulthunter",
	"!rehi": "resulthunter_images",
	"!rel": "reloado",
	"!reloado": "reloado",
	"!resulthunter": "resulthunter",
	"!resulthunter_images": "resulthunter_images",
	"!reu": "reuters",
	"!reuters": "reuters",
	"!rottentomatoes": "rottentomatoes",
	"!rt": "rottentomatoes",
	"!ru": "rumble",
	"!rubygems": "rubygems",
	"!rumble": "rumble",
	"!sc": "soundcloud",
	"!sch": "searchch",
	"!scr": "senscritique",
	"!se": "semantic_scholar",
	"!searchch": "searchch",
	"!searchmysite": "searchmysite",
	"!selfhst_icons": "selfhst_icons",
	"!semantic_scholar": "semantic_scholar",
	"!senscritique": "senscritique",
	"!sep": "sepiasearch",
	"!sepiasearch": "sepiasearch",
	"!seznam": "seznam",
	"!shopify_stock": "shopify_stock",
	"!shs": "shopify_stock",
	"!si": "selfhst_icons",
	"!sms": "searchmysite",
	"!sogou": "sogou",
	"!sogou_images": "sogou_images",
	"!sogou_videos": "sogou_videos",
	"!sogou_wechat": "sogou_wechat",
	"!sogoui": "sogou_images",
	"!sogouv": "sogou_videos",
	"!sogouw": "sogou_wechat",
	"!solid": "solidtorrents",
	"!solidtorrents": "solidtorrents",
	"!soundcloud": "soundcloud",
	"!sourcehut": "sourcehut",
	"!sp": "startpage",
	"!spi": "startpage_images",
	"!spn": "startpage_news",
	"!srht": "sourcehut",
	"!st": "stackoverflow",
	"!stackoverflow": "stackoverflow",
	"!startpage": "startpage",
	"!startpage_images": "startpage_images",
	"!startpage_news": "startpage_news",
	"!steam": "steam",
	"!stm": "steam",
	"!sto": "stocksnap",
	"!stocksnap": "stocksnap",
	"!su": "superuser",
	"!superuser": "superuser",
	"!sw": "swisscows",
	"!swisscows": "swisscows",
	"!szn": "seznam",
	"!tagesschau": "tagesschau",
	"!tin": "tineye",
	"!tineye": "tineye",
	"!tl": "mymemory_translated",
	"!tm": "tmdb",
	"!tmdb": "tmdb",
	"!tokyotoshokan": "tokyotoshokan",
	"!toot": "tootfinder",
	"!tootfinder": "tootfinder",
	"!tpb": "piratebay",
	"!ts": "tagesschau",
	"!tt": "tokyotoshokan",
	"!tu": "tusksearch",
	"!tui": "tusksearch_images",
	"!tun": "tusksearch_news",
	"!tusksearch": "tusksearch",
	"!tusksearch_images": "tusksearch_images",
	"!tusksearch_news": "tusksearch_news",
	"!tusksearch_videos": "tusksearch_videos",
	"!tuv": "tusksearch_videos",
	"!ubuntu": "askubuntu",
	"!unsplash": "unsplash",
	"!us": "unsplash",
	"!ux": "uxwing",
	"!uxwing": "uxwing",
	"!vimeo": "vimeo",
	"!vm": "vimeo",
	"!void": "voidlinux",
	"!voidlinux": "voidlinux",
	"!vu": "vuhuv",
	"!vuhuv": "vuhuv",
	"!vuhuv_images": "vuhuv_images",
	"!vuhuv_videos": "vuhuv_videos",
	"!vui": "vuhuv_images",
	"!vuv": "vuhuv_videos",
	"!wa": "wolframalpha",
	"!wb": "wikibooks",
	"!wd": "wikidata",
	"!wib": "wiby",
	"!wiby": "wiby",
	"!wikibooks": "wikibooks",
	"!wikidata": "wikidata",
	"!wikimini": "wikimini",
	"!wikinews": "wikinews",
	"!wikipedia": "wikipedia",
	"!wikiquote": "wikiquote",
	"!wikisource": "wikisource",
	"!wikispecies": "wikispecies",
	"!wikiversity": "wikiversity",
	"!wikivoyage": "wikivoyage",
	"!wiktionary": "wiktionary",
	"!wkmn": "wikimini",
	"!wn": "wikinews",
	"!wnik": "wordnik",
	"!wolframalpha": "wolframalpha",
	"!wordnik": "wordnik",
	"!wp": "wikipedia",
	"!wq": "wikiquote",
	"!ws": "wikisource",
	"!wsp": "wikispecies",
	"!wt": "wiktionary",
	"!wv": "wikiversity",
	"!wy": "wikivoyage",
	"!ya": "yacy",
	"!yacy": "yacy",
	"!yacy_images": "yacy_images",
	"!yahoo": "yahoo",
	"!yai": "yacy_images",
	"!yandex": "yandex",
	"!yandex_images": "yandex_images",
	"!yandex_music": "yandex_music",
	"!yd": "yandex",
	"!ydi": "yandex_images",
	"!ydm": "yandex_music",
	"!yep": "yep",
	"!yh": "yahoo",
	"!youtube": "youtube",
	"!yt": "youtube",
	"!zapmeta": "zapmeta",
	"!zpm": "zapmeta",
	"!breach": "breach",
	"!leak": "breach",
	"!leaks": "breach",
	"!hibp": "breach",
	"!pwned": "breach",
	"!intelx": "breach",
	"!pastes": "pastes",
	"!paste": "pastes",
	"!pastebin": "pastes",
	"!rentry": "pastes",
	"!dump": "pastes",
	"!dumps": "pastes",
	"!web3": "web3",
	"!ipfs": "web3",
	"!arweave": "web3",
	"!wl": "wikileaks",
	"!wikileaks": "wikileaks",
	"!ddos": "wikileaks",
	"!ddosecrets": "wikileaks",
	"!cryptome": "wikileaks",
	"!foia": "wikileaks",
	"!ransom": "ransomware",
	"!ransomware": "ransomware",
	"!darkfeed": "ransomware",
	"!ransomlook": "ransomware",
	"!ransomlive": "ransomware",
	"!tg": "telegram_leaks",
	"!telegram": "telegram_leaks",
	"!tgdump": "telegram_leaks",
	"!tgstat": "telegram_leaks",
	"!tme": "telegram_leaks",
	"!tlgrm": "telegram_leaks",
	"!lyzem": "telegram_leaks",
	"!telegago": "telegram_leaks",
	"!exploit": "exploits",
	"!exploits": "exploits",
	"!sploitus": "exploits",
	"!packetstorm": "exploits",
	"!edb": "exploits",
	"!cxsecurity": "exploits",
	"!ah": "ahmia",
	"!ahmia": "ahmia",
	"!ads": "astrophysics_data_system",
	"!astrophysics_data_system": "astrophysics_data_system",
	"!az": "azure",
	"!azure": "azure",
	"!base": "base",
	"!bapi": "braveapi",
	"!braveapi": "braveapi",
	"!cachy": "cachy_os",
	"!cachy_os": "cachy_os",
	"!c3tv": "ccc_media",
	"!ccc_media": "ccc_media",
	"!cn": "chatnoir",
	"!chatnoir": "chatnoir",
	"!chinaso": "chinaso",
	"!cfai": "cloudflareai",
	"!cloudflareai": "cloudflareai",
	"!cor": "core",
	"!core": "core",
	"!crates": "crates",
	"!curr": "currency_convert",
	"!currency_convert": "currency_convert",
	"!da": "deviantart",
	"!deviantart": "deviantart",
	"!digbt": "digbt",
	"!disc": "discourse",
	"!discourse": "discourse",
	"!doku": "doku",
	"!dokuwiki": "doku",
	"!eb": "ebay",
	"!ebay": "ebay",
	"!epmc": "europepmc",
	"!europepmc": "europepmc",
	"!exa": "exaapi",
	"!exaapi": "exaapi",
	"!fs": "freesound",
	"!freesound": "freesound",
	"!gitea": "gitea",
	"!ghc": "github_code",
	"!github_code": "github_code",
	"!gp": "google_play",
	"!google_play": "google_play",
	"!grok": "grokipedia",
	"!grokipedia": "grokipedia",
	"!inv": "invidious",
	"!invidious": "invidious",
	"!jina": "jina",
	"!kagi": "kagi",
	"!keen": "keenable",
	"!keenable": "keenable",
	"!lemmy": "lemmy",
	"!librs": "lib_rs",
	"!lib_rs": "lib_rs",
	"!mastodon": "mastodon",
	"!mw": "mediawiki",
	"!mediawiki": "mediawiki",
	"!mrs": "mrs",
	"!neo": "neocities",
	"!neocities": "neocities",
	"!neos": "neosearch",
	"!neosearch": "neosearch",
	"!oca": "openclipart",
	"!openclipart": "openclipart",
	"!oss": "opensemantic",
	"!opensemantic": "opensemantic",
	"!piped": "piped",
	"!pixiv": "pixiv",
	"!pkg": "pkg_go_dev",
	"!pkg_go_dev": "pkg_go_dev",
	"!repo": "repology",
	"!repology": "repology",
	"!s1": "s1search",
	"!s1search": "s1search",
	"!s1r": "s1search_rampjs",
	"!s1search_rampjs": "s1search_rampjs",
	"!scanr": "scanr_structures",
	"!scanr_structures": "scanr_structures",
	"!srock": "searchrockit",
	"!searchrockit": "searchrockit",
	"!szee": "searchzee",
	"!searchzee": "searchzee",
	"!sninja": "seekninja",
	"!seekninja": "seekninja",
	"!selfhst": "selfhst",
	"!spo": "spotify",
	"!spotify": "spotify",
	"!springer": "springer",
	"!stex": "stackexchange",
	"!stackexchange": "stackexchange",
	"!startp": "startpagina",
	"!startpagina": "startpagina",
	"!swn": "swisscows_news",
	"!swisscows_news": "swisscows_news",
	"!tiger": "tiger",
	"!tonline": "tonline",
	"!tz": "torznab",
	"!torznab": "torznab",
	"!ta": "tubearchivist",
	"!tubearchivist": "tubearchivist",
	"!wh": "wallhaven",
	"!wallhaven": "wallhaven",
	"!commons": "wikicommons",
	"!wikicommons": "wikicommons",
	"!yn": "yahoo_news",
	"!yahoo_news": "yahoo_news",
	"!zlib": "zlibrary",
	"!zlibrary": "zlibrary",
}

// Direct bang URL templates (for !bang!)
var directBangURLs = map[string]string{
	"!g!":         "https://www.google.com/search?q=%s",
	"!google!":    "https://www.google.com/search?q=%s",
	"!ddg!":       "https://duckduckgo.com/?q=%s",
	"!yt!":        "https://www.youtube.com/results?search_query=%s",
	"!youtube!":   "https://www.youtube.com/results?search_query=%s",
	"!gh!":        "https://github.com/search?q=%s",
	"!github!":    "https://github.com/search?q=%s",
	"!w!":         "https://en.wikipedia.org/wiki/Special:Search?search=%s",
	"!wiki!":      "https://en.wikipedia.org/wiki/Special:Search?search=%s",
	"!so!":        "https://stackoverflow.com/search?q=%s",
	"!r!":         "https://www.reddit.com/search/?q=%s",
	"!reddit!":    "https://www.reddit.com/search/?q=%s",
	"!map!":       "https://www.openstreetmap.org/search?query=%s",
	"!osm!":       "https://www.openstreetmap.org/search?query=%s",
	"!hn!":        "https://news.ycombinator.com/",
	"!arxiv!":     "https://arxiv.org/search/?query=%s&searchtype=all",
	"!pb!":        "https://thepiratebay.org/search.php?q=%s",
	"!1337x!":     "https://1337x.to/search/%s/1/",
	"!mar!":       "https://marginalia.nu/search?query=%s",
	"!wib!":       "https://wiby.me/?q=%s",
	"!moj!":       "https://www.mojeek.com/search?q=%s",
	"!qw!":        "https://www.qwant.com/?q=%s",
	"!sp!":        "https://www.startpage.com/sp/search?query=%s",
	"!bi!":        "https://www.bing.com/search?q=%s",
	"!br!":        "https://search.brave.com/search?q=%s",
	"!bd!":        "https://www.baidu.com/s?wd=%s",
	"!yd!":        "https://yandex.com/search/?text=%s",
	"!hibp!":      "https://haveibeenpwned.com/unifiedsearch/%s",
	"!pastebin!":  "https://pastebin.com/search?q=%s",
	"!intelx!":    "https://intelx.io/?s=%s",
	"!rentry!":    "https://rentry.co/%s",
	"!ipfs!":      "https://ipfs.io/ipfs/%s",
	"!arweave!":   "https://viewblock.io/arweave/tx/%s",
	"!wl!":        "https://wikileaks.org/wiki/%s",
	"!wikileaks!": "https://wikileaks.org/wiki/%s",
	"!ddos!":      "https://ddosecrets.com/wiki/Special:Search?search=%s",
	"!sploitus!":  "https://sploitus.com/?query=%s",
	"!exploit!":   "https://www.exploit-db.com/search?q=%s",
	"!packetstorm!": "https://packetstormsecurity.com/search/?q=%s",
	"!tgstat!":    "https://tgstat.com/search?q=%s",
	"!telemetr!":  "https://telemetr.io/en/channels?search=%s",
	"!lyzem!":     "https://lyzem.com/search?q=%s",
}

// ParseBangs inspects raw query for bangs (!g, !yt, !images) and language modifiers (:en, :id)
func ParseBangs(rawQuery string, defaultCat models.Category) ParsedBangResult {
	tokens := strings.Fields(strings.TrimSpace(rawQuery))
	if len(tokens) == 0 {
		return ParsedBangResult{Category: defaultCat}
	}

	var remainingTokens []string
	var selectedEngines []string
	var excludedEngines []string
	var excludedSites []string
	siteFilter := ""
	filetype := ""
	intitle := ""
	inurl := ""
	exactPhrase := ""
	category := defaultCat
	language := ""
	directURL := ""
	var timeoutLimit time.Duration

	var dateAfter *time.Time
	var dateBefore *time.Time
	var mustTerms []string
	var mustNotTerms []string
	var orTerms [][]string
	country := ""
	region := ""
	deepSearch := false
	crossCategory := false

	// Extract exact phrase if in quotes: e.g. "exact keywords"
	if strings.Contains(rawQuery, "\"") {
		start := strings.Index(rawQuery, "\"")
		end := strings.LastIndex(rawQuery, "\"")
		if end > start {
			exactPhrase = strings.TrimSpace(rawQuery[start+1 : end])
		}
	}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		lower := strings.ToLower(token)

		// 0. Check Deep / Cross-Category Bangs: !deep, !broad, !cross, !all
		if lower == "!deep" || lower == "!broad" {
			deepSearch = true
			continue
		}
		if lower == "!cross" || lower == "!federated" || lower == "!all" {
			crossCategory = true
			continue
		}

		// 1. Check direct bang: e.g. !yt! or !gh!
		if tmpl, ok := directBangURLs[lower]; ok {
			directURL = tmpl
			continue
		}

		// 2. Check timeout modifier: e.g. <3 (3s) or <850 (850ms) or <2s
		if strings.HasPrefix(lower, "<") && len(lower) > 1 {
			rawVal := strings.TrimPrefix(lower, "<")
			if strings.HasSuffix(rawVal, "ms") {
				if ms, err := strconv.Atoi(strings.TrimSuffix(rawVal, "ms")); err == nil {
					timeoutLimit = time.Duration(ms) * time.Millisecond
					continue
				}
			} else if strings.HasSuffix(rawVal, "s") {
				if s, err := strconv.ParseFloat(strings.TrimSuffix(rawVal, "s"), 64); err == nil {
					timeoutLimit = time.Duration(s * 1000) * time.Millisecond
					continue
				}
			} else if num, err := strconv.Atoi(rawVal); err == nil {
				if num < 100 {
					// SearXNG rule: below 100 is seconds (<3 = 3s)
					timeoutLimit = time.Duration(num) * time.Second
				} else {
					// 100 and above is milliseconds (<850 = 850ms)
					timeoutLimit = time.Duration(num) * time.Millisecond
				}
				continue
			}
		}

		// 3. Check negative engine exclusion: e.g. -!google or !~bing
		if strings.HasPrefix(lower, "-!") || strings.HasPrefix(lower, "!~") {
			cleanBang := "!" + strings.TrimPrefix(strings.TrimPrefix(lower, "-!"), "!~")
			if eng, ok := engineBangs[cleanBang]; ok {
				excludedEngines = append(excludedEngines, eng)
				continue
			}
		}

		// 4. Check category bang or colon prefix: e.g. !images, !vid, !it or :images, :videos, :it, :science
		catKey := lower
		if strings.HasPrefix(catKey, ":") {
			catKey = "!" + strings.TrimPrefix(catKey, ":")
		}
		if cat, ok := categoryBangs[catKey]; ok {
			category = cat
			continue
		}

		// 5. Check engine bang: e.g. !ddg, !gh, !yt
		if eng, ok := engineBangs[lower]; ok {
			selectedEngines = append(selectedEngines, eng)
			continue
		}

		// 6. Check language modifier: e.g. :en, :id, :de, :fr, :ja, :es, :zh, :all or lang:id / language:en
		if strings.HasPrefix(lower, "lang:") && len(lower) > 5 {
			language = strings.TrimPrefix(lower, "lang:")
			continue
		}
		if strings.HasPrefix(lower, "language:") && len(lower) > 9 {
			language = strings.TrimPrefix(lower, "language:")
			continue
		}
		if strings.HasPrefix(lower, ":") && len(lower) >= 3 && len(lower) <= 6 {
			language = strings.TrimPrefix(lower, ":")
			continue
		}

		// 6.1 Check Country & Region modifiers: e.g. country:id or region:id-id
		if strings.HasPrefix(lower, "country:") && len(lower) > 8 {
			country = strings.TrimPrefix(lower, "country:")
			continue
		}
		if strings.HasPrefix(lower, "region:") && len(lower) > 7 {
			region = strings.TrimPrefix(lower, "region:")
			continue
		}

		// 6.2 Check Date Filters: e.g. after:2024-01-01, before:2024-12-31, since:2023, year:2024
		if strings.HasPrefix(lower, "after:") && len(lower) > 6 {
			dateAfter = parseDateFilter(strings.TrimPrefix(lower, "after:"))
			continue
		}
		if strings.HasPrefix(lower, "since:") && len(lower) > 6 {
			dateAfter = parseDateFilter(strings.TrimPrefix(lower, "since:"))
			continue
		}
		if strings.HasPrefix(lower, "before:") && len(lower) > 7 {
			dateBefore = parseDateFilter(strings.TrimPrefix(lower, "before:"))
			continue
		}
		if strings.HasPrefix(lower, "year:") && len(lower) > 5 {
			yStr := strings.TrimPrefix(lower, "year:")
			if y, err := strconv.Atoi(yStr); err == nil && y > 1900 && y < 2100 {
				tStart := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
				tEnd := time.Date(y, 12, 31, 23, 59, 59, 0, time.UTC)
				dateAfter = &tStart
				dateBefore = &tEnd
				continue
			}
		}

		// 7. Check site filter: e.g. site:github.com or -site:pinterest.com
		if strings.HasPrefix(lower, "-site:") && len(lower) > 6 {
			domain := strings.TrimPrefix(lower, "-site:")
			domain = strings.TrimPrefix(domain, "www.")
			excludedSites = append(excludedSites, domain)
			remainingTokens = append(remainingTokens, token)
			continue
		}
		if strings.HasPrefix(lower, "site:") && len(lower) > 5 {
			domain := strings.TrimPrefix(lower, "site:")
			domain = strings.TrimPrefix(domain, "www.")
			siteFilter = domain
			remainingTokens = append(remainingTokens, token)
			continue
		}

		// 8. Check filetype filter: e.g. filetype:pdf or ext:pdf
		if strings.HasPrefix(lower, "filetype:") && len(lower) > 9 {
			filetype = strings.TrimPrefix(lower, "filetype:")
			continue
		}
		if strings.HasPrefix(lower, "ext:") && len(lower) > 4 {
			filetype = strings.TrimPrefix(lower, "ext:")
			continue
		}

		// 9. Check intitle filter: e.g. intitle:tutorial
		if strings.HasPrefix(lower, "intitle:") && len(lower) > 8 {
			intitle = strings.TrimPrefix(lower, "intitle:")
			continue
		}

		// 10. Check inurl filter: e.g. inurl:docs
		if strings.HasPrefix(lower, "inurl:") && len(lower) > 6 {
			inurl = strings.TrimPrefix(lower, "inurl:")
			continue
		}

		// 11. Check Boolean Operators: AND, OR, NOT, +term, -term
		if token == "NOT" && i+1 < len(tokens) {
			next := tokens[i+1]
			mustNotTerms = append(mustNotTerms, strings.ToLower(next))
			i++ // skip next token in raw
			continue
		}
		if token == "OR" && len(remainingTokens) > 0 && i+1 < len(tokens) {
			prev := remainingTokens[len(remainingTokens)-1]
			next := tokens[i+1]
			orTerms = append(orTerms, []string{strings.ToLower(prev), strings.ToLower(next)})
			remainingTokens = append(remainingTokens, next)
			i++ // advance token index
			continue
		}
		if token == "AND" {
			// AND is an explicit conjunction; ensure previous and next tokens are mustTerms
			if len(remainingTokens) > 0 {
				mustTerms = append(mustTerms, strings.ToLower(remainingTokens[len(remainingTokens)-1]))
			}
			continue
		}
		if strings.HasPrefix(token, "+") && len(token) > 1 {
			term := strings.TrimPrefix(token, "+")
			mustTerms = append(mustTerms, strings.ToLower(term))
			remainingTokens = append(remainingTokens, term)
			continue
		}
		if strings.HasPrefix(token, "-") && len(token) > 1 && !strings.HasPrefix(lower, "-site:") && !strings.HasPrefix(lower, "-!") {
			term := strings.TrimPrefix(token, "-")
			mustNotTerms = append(mustNotTerms, strings.ToLower(term))
			continue
		}

		remainingTokens = append(remainingTokens, token)
	}

	cleanQuery := strings.Join(remainingTokens, " ")

	// If direct bang was found, construct destination URL
	var fullDirectURL string
	if directURL != "" {
		if strings.Contains(directURL, "%s") {
			fullDirectURL = fmt.Sprintf(directURL, url.QueryEscape(cleanQuery))
		} else {
			fullDirectURL = directURL
		}
	}

	return ParsedBangResult{
		CleanQuery:        cleanQuery,
		Category:          category,
		Engines:           selectedEngines,
		ExcludedEngines:   excludedEngines,
		Language:          language,
		Country:           country,
		Region:            region,
		TimeoutLimit:      timeoutLimit,
		DirectRedirectURL: fullDirectURL,
		SiteFilter:        siteFilter,
		ExcludedSites:     excludedSites,
		Filetype:          filetype,
		Intitle:           intitle,
		Inurl:             inurl,
		ExactPhrase:       exactPhrase,
		DateAfter:         dateAfter,
		DateBefore:        dateBefore,
		MustTerms:         mustTerms,
		MustNotTerms:      mustNotTerms,
		OrTerms:           orTerms,
		DeepSearch:        deepSearch,
		CrossCategory:     crossCategory,
	}
}

func parseDateFilter(val string) *time.Time {
	val = strings.TrimSpace(val)
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"2006-01",
		"2006",
		"02-01-2006",
		"02/01/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, val); err == nil {
			return &t
		}
	}
	return nil
}

// SuggestBangs returns matching category, engine, and direct bangs for search autocompletion
func SuggestBangs(prefix string, limit int) []string {
	if limit <= 0 {
		limit = 8
	}
	p := strings.ToLower(strings.TrimSpace(prefix))
	var matches []string

	// 1. Category bangs
	for bang := range categoryBangs {
		if strings.HasPrefix(bang, p) {
			matches = append(matches, bang)
			if len(matches) >= limit {
				return matches
			}
		}
	}

	// 2. Engine bangs
	for bang, engine := range engineBangs {
		if strings.HasPrefix(bang, p) || strings.HasPrefix("!"+engine, p) {
			matches = append(matches, bang+" ("+engine+")")
			if len(matches) >= limit {
				return matches
			}
		}
	}

	return matches
}

