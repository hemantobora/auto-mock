
package repl

import (
    "fmt"
    "strings"

    "github.com/AlecAivazis/survey/v2"
    "github.com/hemantobora/auto-mock/internal/utils"
)

func ResolveProjectInteractively(existing []string) (string, bool, error) {
    if len(existing) == 0 {
        name, err := promptNewProject(existing)
        return name, false, err
    }

    baseProjects := []string{}
    for _, b := range existing {
        trimmed := utils.RemoveBucketPrefix(b)
        parts := strings.Split(trimmed, "-")
        if len(parts) < 2 {
            continue // invalid name, skip
        }
        project := strings.Join(parts[:len(parts)-1], "-")
        if project != "" {
            baseProjects = append(baseProjects, project)
        }
    }

    baseProjects = append(baseProjects, "Create New Project")

    var choice string
    prompt := &survey.Select{
        Message: "Choose an existing project or create a new one:",
        Options: baseProjects,
    }

    if err := survey.AskOne(prompt, &choice); err != nil {
        return "", false, err
    }

    if choice == "Create New Project" {
        name, err := promptNewProject(existing)
        return name, false, err
    }

    for _, b := range existing {
    trimmed := utils.RemoveBucketPrefix(b)
        parts := strings.Split(trimmed, "-")
        base := strings.Join(parts[:len(parts)-1], "-")
        if base == choice {
            return trimmed, true, nil
        }
    }

    return "", false, fmt.Errorf("selected project not found")
}

func promptNewProject(existing []string) (string, error) {
    for {
        var name string
        if err := survey.AskOne(&survey.Input{Message: "Enter a name for your new project:"}, &name); err != nil {
            return "", err
        }

        if isProjectNameTaken(name, existing) {
            fmt.Println("⚠️ A project with that name already exists (case-insensitive). Please choose a different name.")
            continue
        }

        suffix, err := utils.GenerateRandomSuffix()
        if err != nil {
            return "", err
        }

        return fmt.Sprintf("%s-%s", name, suffix), nil
    }
}

func isProjectNameTaken(name string, existing []string) bool {
    input := strings.ToLower(name)
    for _, bucket := range existing {
    project := utils.RemoveBucketPrefix(bucket)
        parts := strings.Split(project, "-")
        base := strings.Join(parts[:len(parts)-1], "-")
        if strings.ToLower(base) == input {
            return true
        }
    }
    return false
}

func SelectProjectAction(project string) string {
    var action string
    prompt := &survey.Select{
        Message: fmt.Sprintf("What would you like to do with project '%s'?", project),
        Options: []string{"Edit stubs (coming soon)", "Delete project", "Cancel"},
    }
    survey.AskOne(prompt, &action)
    return strings.Split(action, " ")[0] // "Edit", "Delete", "Cancel"
}
