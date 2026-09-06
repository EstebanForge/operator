// Package tmux provides client communication and platform utilities for tmux.
package tmux

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// EnglishWords contains 500 curated English words for session naming.
var EnglishWords = []string{
	"acorn", "alder", "amber", "anchor", "ant", "anvil", "apex", "arc", "arch", "armor",
	"arrow", "ash", "aspen", "atlas", "aurora", "axis", "badge", "badger", "bamboo", "banner",
	"basalt", "bastion", "battery", "bay", "beacon", "beam", "bear", "beaver", "beech", "beetle",
	"bell", "birch", "bison", "blade", "blaze", "blizzard", "block", "board", "bolt", "bond",
	"boom", "boost", "boot", "boulder", "bound", "brace", "bracket", "bramble", "branch", "brand",
	"brass", "breeze", "bridge", "brier", "bright", "bronze", "brook", "buffer", "cable", "cadence",
	"cage", "camel", "camp", "canvas", "canyon", "cape", "capsule", "carbon", "card", "castle",
	"cave", "cedar", "chain", "channel", "charm", "chart", "chassis", "cheetah", "chill", "chip",
	"chord", "chrome", "cipher", "circuit", "citadel", "clamp", "clasp", "claw", "clay", "clef",
	"cliff", "cloak", "clock", "cloud", "clover", "cluster", "coal", "coast", "cobra", "code",
	"coil", "column", "comet", "compass", "conduit", "cone", "console", "copper", "coral", "core",
	"corona", "cougar", "cove", "coyote", "craft", "crag", "crane", "crate", "creek", "crest",
	"cross", "crow", "crown", "crystal", "cube", "cure", "current", "cursor", "cyclone", "cypress",
	"dale", "dart", "dash", "dawn", "deck", "deer", "delta", "desert", "dial", "diamond",
	"digit", "disc", "dock", "dolphin", "dome", "door", "draft", "dragon", "drift", "drizzle",
	"drone", "drum", "dune", "dusk", "dynamo", "eagle", "echo", "eclipse", "edge", "elk",
	"elm", "ember", "emerald", "engine", "epoch", "facet", "falcon", "fern", "ferret", "fiber",
	"field", "filament", "file", "filter", "finch", "fir", "flame", "flange", "flare", "flash",
	"flask", "fleet", "flint", "flood", "flora", "flurry", "flux", "focus", "fog", "font",
	"force", "forest", "forge", "form", "fort", "fossil", "fox", "frame", "freeze", "frost",
	"fuse", "galaxy", "gale", "garnet", "gauge", "gazelle", "gear", "gecko", "gem", "geyser",
	"gibbon", "gimbal", "giraffe", "glacier", "glade", "glass", "glaze", "glen", "glide", "glow",
	"glyph", "gong", "gradient", "granite", "grid", "groove", "grove", "guild", "gulf", "gull",
	"gust", "hail", "halo", "hammer", "handle", "harbor", "hare", "hatch", "haven", "hawk",
	"haze", "hazel", "heart", "heather", "hedgehog", "helix", "helm", "heron", "hill", "hinge",
	"hollow", "hook", "horizon", "hornet", "hound", "hub", "hull", "hyena", "ice", "icon",
	"iguana", "index", "inlay", "inlet", "intake", "iron", "island", "ivy", "jade", "jaguar",
	"jasper", "jay", "joint", "jungle", "keel", "kelp", "kernel", "key", "kite", "knife",
	"knob", "knot", "koala", "label", "lagoon", "lake", "lance", "lantern", "larch", "lark",
	"laser", "latch", "lattice", "lava", "layer", "lead", "leaf", "ledge", "lemur", "lens",
	"leopard", "lever", "lichen", "light", "lightning", "lily", "lime", "link", "lion", "lizard",
	"llama", "loam", "lock", "loom", "loop", "lunar", "lynx", "macro", "magnet", "magpie",
	"mantle", "maple", "marlin", "marsh", "marten", "matrix", "meadow", "mercury", "mesa", "mesh",
	"meteor", "mica", "mist", "module", "mole", "monitor", "monkey", "monsoon", "moon", "moose",
	"morning", "mortar", "moss", "motor", "mount", "mountain", "nebula", "needle", "nerve", "nest",
	"nexus", "night", "node", "noon", "nova", "nozzle", "nucleus", "oak", "oasis", "ocean",
	"ocelot", "opal", "optic", "orbit", "orca", "ore", "osprey", "otter", "owl", "packet",
	"paddle", "palm", "panda", "panel", "panther", "parrot", "patch", "peak", "pebble", "pedal",
	"peg", "pelican", "pendulum", "penguin", "phase", "pheasant", "pigeon", "pike", "pillar", "pilot",
	"pin", "pine", "pipe", "piston", "pivot", "pixel", "planet", "plasma", "plate", "plateau",
	"plow", "plug", "plume", "pocket", "point", "polar", "pole", "pond", "pony", "poplar",
	"port", "portal", "prairie", "prism", "probe", "prong", "prop", "puffin", "pulley", "pulsar",
	"pulse", "puma", "pump", "pyramid", "quail", "quartz", "quasar", "quiver", "rabbit", "raccoon",
	"radar", "radio", "rail", "rain", "rainbow", "ram", "ramp", "range", "raven", "ravine",
	"ray", "reef", "relay", "render", "resistor", "ribbon", "ridge", "ring", "river", "rivet",
	"robin", "robot", "rock", "rod", "root", "rose", "rotor", "router", "rover", "ruby",
	"rudder", "rune", "safe", "sage", "sail", "salamander", "salmon", "sand", "savanna", "scale",
	"scan", "schema", "scope", "screen", "screw", "sea", "seal", "sector", "sediment", "sensor",
	"serum", "server", "shadow", "shaft", "shale", "shard", "shark", "shield", "shift", "shine",
	"ship", "shore", "shower", "shuttle", "signal", "silver", "siren", "skiff", "sky", "slab",
}

// SpanishWords contains 500 curated Spanish words for session naming.
var SpanishWords = []string{
	"abaco", "abeja", "abismo", "acantilado", "adelfa", "aguila", "aire", "alambre", "alameda", "alarma",
	"alba", "albedo", "alborada", "alcazar", "alce", "alcor", "aldea", "alea", "aleacion", "alfiler",
	"alga", "algoritmo", "alicate", "alisio", "aliso", "almena", "alondra", "altavoz", "altiplano", "aluminio",
	"ambar", "amuleto", "anchoa", "ancla", "anguila", "anillo", "antena", "apogeo", "arbol", "arboleda",
	"archipielago", "archivo", "arco", "ardilla", "arena", "armadillo", "armazon", "arnes", "arpon", "arrecife",
	"arroyo", "arsenal", "artefacto", "asfalto", "astil", "astro", "astronave", "atalaya", "atmosfera", "atril",
	"atun", "aurora", "autopista", "avestruz", "avispa", "bahia", "bala", "baliza", "ballena", "balsa",
	"banco", "banda", "bandera", "barca", "barco", "barra", "barranco", "barril", "barro", "basalto",
	"base", "baston", "bateria", "bisonte", "blindaje", "bloque", "bobina", "boceto", "bochorno", "bodega",
	"bomba", "bordo", "borne", "borrasca", "bosque", "boton", "boya", "bravo", "brazo", "brezo",
	"brida", "brisa", "bronce", "brujula", "bucle", "bufalo", "buho", "buque", "burbuja", "caballo",
	"cable", "cabra", "cadena", "calamar", "caleta", "calzada", "calima", "calma", "calor", "camaleon",
	"camara", "camello", "campana", "campo", "canal", "canario", "canguro", "cantera", "canula", "capa",
	"capsula", "caracol", "carbon", "carena", "carro", "carta", "cartel", "cascada", "casco", "castillo",
	"castor", "catalejo", "cauce", "caverna", "cebra", "cedro", "celda", "cemento", "cenador", "cenit",
	"cenote", "centella", "cepillo", "cerro", "cerrojo", "chaparral", "charco", "chispa", "ciervo", "cifrado",
	"cilindro", "cima", "cinta", "cipres", "circuito", "cirro", "cisne", "ciudadela", "clase", "clave",
	"clavel", "clavo", "clima", "coati", "cobra", "cobre", "codigo", "cohete", "cojinete", "colibri",
	"colina", "columna", "cometa", "compas", "compuerta", "condor", "conejo", "conexion", "consola", "constelacion",
	"contacto", "coral", "cordillera", "cordon", "corona", "correa", "cosmos", "costa", "cotorra", "crater",
	"crepusculo", "cresta", "crisol", "cristal", "cronometro", "cubierta", "cubo", "cuero", "cuesta", "cueva",
	"cumbre", "cumulo", "cupula", "dado", "danta", "dardo", "dehesa", "delfin", "delta", "desierto",
	"diluvio", "dique", "disco", "doblador", "domo", "dorado", "duela", "duna", "eclipse", "eje",
	"electrodo", "embudo", "emisor", "empalme", "encina", "engranaje", "enlace", "ensenada", "erizo", "escala",
	"escalera", "escarabajo", "esclusa", "escorpion", "escudo", "eslabon", "esmeralda", "espada", "espatula", "espejo",
	"espiral", "espolon", "espuela", "estacion", "estante", "estator", "estela", "estepa", "estrella", "estribo",
	"estuario", "faisan", "faro", "fibra", "fieltro", "filamento", "filete", "filtro", "fiordo", "firmamento",
	"flamenco", "flecha", "flor", "floresta", "flotador", "foca", "foco", "fogata", "forja", "formula",
	"fortin", "freno", "frio", "fronda", "fuelle", "fuente", "fuerza", "fulgor", "funda", "fuselaje",
	"fusible", "gacela", "galaxia", "galga", "galgo", "gancho", "garita", "garza", "gaveta", "gaviota",
	"gema", "giroscopio", "glaciar", "golfo", "granizo", "grillete", "grua", "gruta", "guepardo", "guia",
	"halcon", "halo", "hebra", "helada", "helice", "herraje", "hiena", "hierro", "higuera", "hilera",
	"horizonte", "huerto", "huracan", "huron", "huso", "iguana", "iman", "impulso", "indice", "invierno",
	"isla", "islote", "jabali", "jaguar", "jardin", "jungla", "junta", "koala", "lagarto", "lago",
	"laguna", "lamina", "lampara", "langosta", "lanza", "laser", "lata", "laton", "laurel", "lechuza",
	"lente", "leopardo", "leva", "lienzo", "limo", "lince", "lingote", "linterna", "llama", "llano",
	"llanura", "llave", "llovizna", "lluvia", "lobo", "loma", "lona", "loro", "lucero", "luciernaga",
	"luna", "macizo", "madreselva", "magia", "malla", "manantial", "mandril", "manglar", "manivela", "maquina",
	"mar", "marca", "marco", "marea", "marisma", "marmota", "martillo", "mastil", "matorral", "matraz",
	"matriz", "mecha", "medano", "medusa", "membrana", "meseta", "meteorito", "meteoro", "milano", "mineral",
	"mirador", "modulo", "molde", "molino", "mono", "monte", "morrena", "morsa", "muelle", "muralla",
	"musgo", "mustela", "naranja", "navaja", "nave", "nebula", "nebulosa", "nevisca", "niebla", "nieve",
	"nimbo", "noche", "nodo", "norma", "norte", "nubarrada", "nube", "nucleo", "nutria", "oasis",
	"obelisco", "ocaso", "oceano", "ocelote", "olivo", "olmo", "onda", "orbita", "oriente", "orilla",
	"oro", "oropendola", "oso", "otono", "palanca", "palmera", "paloma", "panel", "pantalla", "pantano",
	"pantera", "paraje", "paramo", "pararrayos", "parche", "pardo", "parque", "pasador", "paso", "pastizal",
	"pato", "patron", "pedal", "pedregal", "pelicano", "pelicula", "pendulo", "peninsula", "penumbra", "peon",
	"perdiz", "perfil", "perigeo", "perno", "picaflor", "pico", "pila", "pilar", "pilon", "pinar",
	"pino", "pinza", "piston", "pivot", "pivote", "placa", "planeta", "plasma", "plata", "plataforma",
	"playa", "plomo", "polea", "poligono", "polo", "polvo", "poniente", "porton", "poste", "potencia",
}

var combinedWordPool []string

func init() {
	seen := make(map[string]bool)
	for _, w := range EnglishWords {
		if !seen[w] {
			seen[w] = true
			combinedWordPool = append(combinedWordPool, w)
		}
	}
	for _, w := range SpanishWords {
		if !seen[w] {
			seen[w] = true
			combinedWordPool = append(combinedWordPool, w)
		}
	}
}

// GenerateSessionName generates a 3-word random session name combining English and Spanish words.
func GenerateSessionName() string {
	n := len(combinedWordPool)
	if n < 3 {
		return "session-auto-1"
	}

	selected := make([]string, 0, 3)
	chosenIndices := make(map[int64]bool)
	limit := big.NewInt(int64(n))

	for len(selected) < 3 {
		idxBig, err := rand.Int(rand.Reader, limit)
		if err != nil {
			// Fallback index
			idxBig = big.NewInt(int64(len(selected)))
		}
		idx := idxBig.Int64()
		if !chosenIndices[idx] {
			chosenIndices[idx] = true
			selected = append(selected, combinedWordPool[idx])
		}
	}

	return strings.Join(selected, "-")
}

// GenerateUniqueSessionName generates a 3-word random session name that does not conflict with existing sessions.
func GenerateUniqueSessionName(ctx context.Context, client Client) string {
	for range 10 {
		candidate := GenerateSessionName()
		if client == nil {
			return candidate
		}
		exists, err := client.HasSession(ctx, candidate)
		if err == nil && !exists {
			return candidate
		}
	}

	// In the unlikely case of repeated collision, append a random suffix
	suffix, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return fmt.Sprintf("%s-001", GenerateSessionName())
	}
	return fmt.Sprintf("%s-%03d", GenerateSessionName(), suffix.Int64())
}
