package db

type Config struct {
	File           string `yaml:"file"`
	BackupSchedule string `yaml:"backupSchedule"`
	BackupDir      string `yaml:"backupDir"`
	BackupNum      int    `yaml:"backupNum"`
}
