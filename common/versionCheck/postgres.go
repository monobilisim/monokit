package common

func PostgresCheck() {
	version := GatherVersion("postgres")

	if version != "" {
		addToNotUpdated(AppVersion{Name: "PostgreSQL", OldVersion: version})
	} else {
		addToNotInstalled("PostgreSQL")
	}
}
