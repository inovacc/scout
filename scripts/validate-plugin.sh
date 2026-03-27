#!/usr/bin/env bash
# Validate the Claude Code plugin structure and content.
set -euo pipefail

ERRORS=0
WARNINGS=0
skill_count=0
agent_count=0

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; ERRORS=$((ERRORS + 1)); }
warn() { echo "  ! $1"; WARNINGS=$((WARNINGS + 1)); }

echo "=== Scout Claude Code Plugin Validation ==="
echo

# 1. Check plugin.json
echo "[manifest]"
if [ -f ".claude-plugin/plugin.json" ]; then
    # Validate JSON syntax
    if python3 -c "import json; json.load(open('.claude-plugin/plugin.json'))" 2>/dev/null; then
        pass "plugin.json: valid JSON"
    else
        fail "plugin.json: invalid JSON"
    fi

    # Check required field: name
    if python3 -c "
import json
d = json.load(open('.claude-plugin/plugin.json'))
assert 'name' in d, 'missing name'
print(f'name={d[\"name\"]}')
" 2>/dev/null; then
        pass "plugin.json: has required 'name' field"
    else
        fail "plugin.json: missing 'name' field"
    fi

    # Check recommended fields
    for field in version description author repository license keywords; do
        if python3 -c "
import json
d = json.load(open('.claude-plugin/plugin.json'))
assert '$field' in d
" 2>/dev/null; then
            pass "plugin.json: has '$field'"
        else
            warn "plugin.json: missing recommended field '$field'"
        fi
    done
else
    fail "plugin.json: file not found at .claude-plugin/plugin.json"
fi

echo

# 2. Check .mcp.json
echo "[mcp]"
if [ -f ".mcp.json" ]; then
    if python3 -c "
import json
d = json.load(open('.mcp.json'))
assert 'mcpServers' in d, 'missing mcpServers'
for name, cfg in d['mcpServers'].items():
    assert 'command' in cfg, f'server {name} missing command'
    print(f'server={name} command={cfg[\"command\"]}')
" 2>/dev/null; then
        pass ".mcp.json: valid with server config"
    else
        fail ".mcp.json: invalid structure"
    fi
else
    warn ".mcp.json: not found (no MCP servers)"
fi

echo

# 3. Check skills
echo "[skills]"
if [ -d "skills" ]; then
    for skill_dir in skills/*/; do
        [ -d "$skill_dir" ] || continue
        skill_name=$(basename "$skill_dir")
        skill_file="${skill_dir}SKILL.md"
        if [ -f "$skill_file" ]; then
            # Check frontmatter
            if head -1 "$skill_file" | grep -q "^---"; then
                # Check description in frontmatter
                if sed -n '/^---$/,/^---$/p' "$skill_file" | grep -q "description:"; then
                    pass "skill '${skill_name}': valid SKILL.md with description"
                    skill_count=$((skill_count + 1))
                else
                    fail "skill '${skill_name}': SKILL.md missing 'description' in frontmatter"
                fi
            else
                fail "skill '${skill_name}': SKILL.md missing frontmatter (---)"
            fi
        else
            fail "skill '${skill_name}': missing SKILL.md"
        fi
    done
    echo "  Total skills: ${skill_count}"
else
    warn "skills/: directory not found"
fi

echo

# 4. Check agents
echo "[agents]"
if [ -d "agents" ]; then
    for agent_file in agents/*.md; do
        [ -f "$agent_file" ] || continue
        agent_name=$(basename "$agent_file" .md)
        if head -1 "$agent_file" | grep -q "^---"; then
            if sed -n '/^---$/,/^---$/p' "$agent_file" | grep -q "description:"; then
                pass "agent '${agent_name}': valid with description"
                agent_count=$((agent_count + 1))
            else
                fail "agent '${agent_name}': missing 'description' in frontmatter"
            fi
        else
            fail "agent '${agent_name}': missing frontmatter"
        fi
    done
    echo "  Total agents: ${agent_count}"
else
    warn "agents/: directory not found"
fi

echo

# 5. Check hooks
echo "[hooks]"
if [ -f "hooks/hooks.json" ]; then
    if python3 -c "
import json
d = json.load(open('hooks/hooks.json'))
assert 'hooks' in d, 'missing hooks key'
for event, handlers in d['hooks'].items():
    print(f'event={event} handlers={len(handlers)}')
" 2>/dev/null; then
        pass "hooks.json: valid"
    else
        fail "hooks.json: invalid structure"
    fi
else
    warn "hooks/hooks.json: not found"
fi

echo

# 6. Check scripts referenced by hooks
echo "[scripts]"
if [ -f "scripts/check-scout.sh" ]; then
    pass "check-scout.sh: exists"
    if [ -x "scripts/check-scout.sh" ] || head -1 "scripts/check-scout.sh" | grep -q "^#!"; then
        pass "check-scout.sh: has shebang"
    else
        warn "check-scout.sh: not executable and no shebang"
    fi
else
    warn "scripts/check-scout.sh: not found"
fi

echo

# 7. Check scout binary builds
echo "[build]"
if command -v go &>/dev/null; then
    if go build ./cmd/scout/ 2>/dev/null; then
        pass "scout binary: builds successfully"
    else
        fail "scout binary: build failed"
    fi
else
    warn "go: not on PATH, skipping build check"
fi

echo
echo "=== Results ==="
echo "  Errors:   ${ERRORS}"
echo "  Warnings: ${WARNINGS}"
echo "  Skills:   ${skill_count}"
echo "  Agents:   ${agent_count}"

if [ "${ERRORS}" -gt 0 ]; then
    echo
    echo "FAILED: ${ERRORS} error(s) found"
    exit 1
fi

echo
echo "PASSED: plugin is valid"
exit 0
