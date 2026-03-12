package templates

type SelectOption struct {
	Value    string
	Label    string
	Selected bool
}

func primaryButtonClass(fullWidth bool) string {
	className := "inline-flex items-center justify-center rounded-xl bg-finops-900 px-4 py-3 text-sm font-extrabold uppercase tracking-[0.16em] text-white shadow-lg shadow-finops-900/20 transition hover:bg-finops-800 focus:outline-none focus:ring-4 focus:ring-finops-400/25"
	if fullWidth {
		return className + " w-full"
	}

	return className + " px-5"
}
