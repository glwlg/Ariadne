package skills

import (
	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func loadSkillsFromSQLite(path string) ([]Skill, bool, error) {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()
	if err := ensureSkillSchema(conn); err != nil {
		return nil, false, err
	}
	skills, ok, err := readSkills(conn)
	if err != nil || ok {
		return skills, ok, err
	}
	var payload stateFile
	if loaded, err := appdb.LegacyLoadJSON(path, "skills", &payload); err == nil && loaded {
		skills = normalizeSkills(payload.Skills)
		if err := saveSkillsToSQLite(path, skills); err != nil {
			return nil, false, err
		}
		_ = appdb.DropLegacyDocument(path, "skills")
		return skills, true, nil
	} else if err != nil {
		var legacy []Skill
		if loaded, legacyErr := appdb.LegacyLoadJSON(path, "skills", &legacy); legacyErr != nil || !loaded {
			return nil, false, err
		}
		skills = normalizeSkills(legacy)
		if err := saveSkillsToSQLite(path, skills); err != nil {
			return nil, false, err
		}
		_ = appdb.DropLegacyDocument(path, "skills")
		return skills, true, nil
	}
	return nil, false, nil
}

func saveSkillsToSQLite(path string, skills []Skill) error {
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := ensureSkillSchema(conn); err != nil {
		return err
	}
	return appdb.Immediate(conn, func() error {
		if err := sqlitex.Execute(conn, `DELETE FROM skill_evidence`, nil); err != nil {
			return err
		}
		if err := sqlitex.Execute(conn, `DELETE FROM skills`, nil); err != nil {
			return err
		}
		for _, skill := range skills {
			if err := insertSkill(conn, skill); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureSkillSchema(conn *sqlite.Conn) error {
	return sqlitex.ExecScript(conn, `
CREATE TABLE IF NOT EXISTS skills(
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS skill_evidence(
  skill_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY(skill_id, position),
  FOREIGN KEY(skill_id) REFERENCES skills(id) ON DELETE CASCADE
);
`)
}

func readSkills(conn *sqlite.Conn) ([]Skill, bool, error) {
	count := 0
	if err := sqlitex.Execute(conn, `SELECT count(*) FROM skills`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error { count = stmt.ColumnInt(0); return nil },
	}); err != nil || count == 0 {
		return nil, false, err
	}
	skills := make([]Skill, 0, count)
	err := sqlitex.Execute(conn, `SELECT id, title, body, source, created_at, updated_at FROM skills`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			skills = append(skills, Skill{
				ID:        stmt.ColumnText(0),
				Title:     stmt.ColumnText(1),
				Body:      stmt.ColumnText(2),
				Source:    stmt.ColumnText(3),
				CreatedAt: stmt.ColumnInt64(4),
				UpdatedAt: stmt.ColumnInt64(5),
			})
			return nil
		},
	})
	if err != nil {
		return nil, false, err
	}
	for index := range skills {
		evidence, err := readSkillEvidence(conn, skills[index].ID)
		if err != nil {
			return nil, false, err
		}
		skills[index].Evidence = evidence
	}
	return normalizeSkills(skills), true, nil
}

func insertSkill(conn *sqlite.Conn, skill Skill) error {
	skill, ok := normalizeSkill(skill)
	if !ok {
		return nil
	}
	if err := sqlitex.Execute(conn, `INSERT INTO skills(id, title, body, source, created_at, updated_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6)`, &sqlitex.ExecOptions{
		Args: []any{skill.ID, skill.Title, skill.Body, skill.Source, skill.CreatedAt, skill.UpdatedAt},
	}); err != nil {
		return err
	}
	for position, value := range skill.Evidence {
		if value == "" {
			continue
		}
		if err := sqlitex.Execute(conn, `INSERT INTO skill_evidence(skill_id, position, value) VALUES (?1, ?2, ?3)`, &sqlitex.ExecOptions{
			Args: []any{skill.ID, position, value},
		}); err != nil {
			return err
		}
	}
	return nil
}

func readSkillEvidence(conn *sqlite.Conn, skillID string) ([]string, error) {
	values := []string{}
	err := sqlitex.Execute(conn, `SELECT value FROM skill_evidence WHERE skill_id = ?1 ORDER BY position`, &sqlitex.ExecOptions{
		Args: []any{skillID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			values = append(values, stmt.ColumnText(0))
			return nil
		},
	})
	return values, err
}
