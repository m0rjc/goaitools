# Contributing to goaitools

Thank you for your interest in contributing to goaitools! This document outlines our development workflow and documentation practices.

## Story-Based Development

We use a **story-based approach** to document feature development, design decisions, and implementation details. This helps maintain context during development while keeping the released library clean.

### What is a Story?

A story is a self-contained documentation folder that captures:
- The motivation and requirements for a feature
- Design decisions and alternatives considered
- Implementation planning and tasks
- Learnings and gotchas discovered during development

### Story Lifecycle

1. **During Development**: Story folders live in `docs/story-NNN-feature-name/`
2. **Before Merge**: Story directories are **deleted** from the main branch
3. **After Release**: Stories remain in git history for future reference

This approach provides:
- **Rich context during development**: All design decisions and rationale documented
- **Clean releases**: Published module doesn't include internal development docs
- **Preserved history**: Git history maintains the full story for future contributors

### Creating a Story

When starting work on a new feature:

This is the layout preferred by AI tooling like Claude if used. This makes it easier to work with.
We're not keeping stories in the codebase, so we cannot track story numbers in a central way.

*To Be Determined* We could use GIT Issue numbers? Or just 001, 002 etc. for stories within a feature
branch. Long-running features are hard to maintain, so there should be few stories.

```bash
# Create story directory
mkdir -p docs/story-NNN-feature-name

# Create required files
touch docs/story-NNN-feature-name/specification.md
touch docs/story-NNN-feature-name/planning.md
```

**Required Files:**

#### `specification.md`
Documents **what** we're building and **why**:
- Problem statement
- Requirements
- User stories or use cases
- Success criteria
- Out of scope items

**Optional Files:**

These tend to be made by AI tools such as Claude.

#### `planning.md`
Documents **how** we'll build it:
- Technical approach
- Architecture decisions
- Task breakdown
- Alternative approaches considered
- Dependencies and risks

#### `implementation-notes.md`
Captures learnings during implementation:
- Unexpected gotchas
- Performance considerations
- Testing insights
- Refactoring notes

### Example Story Structure

```
docs/story-023-stateful-conversations/
├── specification.md          # What: Multi-turn conversation support
├── planning.md               # How: Opaque state management design
└── implementation-notes.md   # Learnings: Token optimization insights
```

### Before Submitting a PR

**Critical**: Delete the story directory before merging to main:
The story can remain through early stages, but by time you merge it should be clear from current documentation
what the system does.

```bash
# Remove story directory
git rm -r docs/story-NNN-feature-name

# Commit the deletion
git commit -m "Remove story-NNN docs before merge"
```

The story documentation will remain accessible in git history:
```bash
# View deleted story files
git log --all --full-history -- "docs/story-023-*"
```

## Development Workflow

1. **Create a feature branch**: `git checkout -b feature/my-feature`
2. **Create story documentation**: `docs/story-NNN-feature-name/`
3. **Implement the feature**: Follow story planning
4. **Update main documentation**: Reflect changes in `README.md`, `CLAUDE.md`, etc.
5. **Delete story directory**: `git rm -r docs/story-NNN-feature-name`
6. **Submit PR**: Include reference to story in commit history

## Code Style Guidelines

- Follow standard Go formatting (`go fmt`)
- Write clear, self-documenting code
- Add comments for non-obvious design decisions
- Keep packages focused and cohesive
- Maintain zero external dependencies (standard library only)

## Testing Requirements

- All new features must include tests
- Run `go test ./...` before submitting
- Aim for high test coverage on public APIs
- Use table-driven tests where appropriate

## Documentation Standards

### Code Documentation
- Public APIs must have godoc comments
- Include usage examples for complex features
- Document error conditions and edge cases

### README.md
- Keep examples up-to-date with API changes
- Update feature list when adding capabilities
- Maintain clear quick-start guide

### Architecture Documentation
Persistent architectural documentation lives in `docs/architecture/`:
- High-level design documents
- Interface contracts
- Cross-cutting concerns
- Long-term technical vision

**Unlike stories**, architecture docs are **not deleted** before merge.

## Questions or Feedback?

Open an issue on GitHub to discuss:
- Feature proposals
- Design questions
- Process improvements
- Documentation clarity

---

**Remember**: Stories are temporary development artifacts. Architecture docs are permanent reference material. Choose the right home for your documentation.
