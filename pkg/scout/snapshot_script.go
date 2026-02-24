package scout

// snapshotJS walks the DOM and produces a YAML-like accessibility tree with ref markers.
const snapshotJS = `(function(config) {
	var refCounter = 0;
	var gen = config.generation || 1;

	function getRole(el) {
		if (el.getAttribute && el.getAttribute('role')) return el.getAttribute('role');
		var tag = el.tagName ? el.tagName.toLowerCase() : '';
		var roleMap = {
			'a': el.hasAttribute && el.hasAttribute('href') ? 'link' : '',
			'article': 'article', 'aside': 'complementary', 'button': 'button',
			'details': 'group', 'dialog': 'dialog', 'footer': 'contentinfo',
			'form': 'form', 'h1': 'heading', 'h2': 'heading', 'h3': 'heading',
			'h4': 'heading', 'h5': 'heading', 'h6': 'heading', 'header': 'banner',
			'hr': 'separator', 'img': 'img', 'input': getInputRole(el),
			'li': 'listitem', 'main': 'main', 'menu': 'menu', 'nav': 'navigation',
			'ol': 'list', 'option': 'option', 'progress': 'progressbar',
			'section': 'region', 'select': 'combobox', 'summary': 'button',
			'table': 'table', 'tbody': 'rowgroup', 'td': 'cell', 'textarea': 'textbox',
			'tfoot': 'rowgroup', 'th': 'columnheader', 'thead': 'rowgroup',
			'tr': 'row', 'ul': 'list'
		};
		return roleMap[tag] || '';
	}

	function getInputRole(el) {
		if (!el.getAttribute) return 'textbox';
		var t = (el.getAttribute('type') || 'text').toLowerCase();
		var map = {
			'button': 'button', 'checkbox': 'checkbox', 'email': 'textbox',
			'number': 'spinbutton', 'password': 'textbox', 'radio': 'radio',
			'range': 'slider', 'search': 'searchbox', 'submit': 'button',
			'tel': 'textbox', 'text': 'textbox', 'url': 'textbox'
		};
		return map[t] || 'textbox';
	}

	function getName(el) {
		if (!el.getAttribute) return '';
		var label = el.getAttribute('aria-label');
		if (label) return label;
		var labelledBy = el.getAttribute('aria-labelledby');
		if (labelledBy) {
			var parts = labelledBy.split(/\s+/).map(function(id) {
				var ref = document.getElementById(id);
				return ref ? ref.textContent.trim() : '';
			}).filter(Boolean);
			if (parts.length) return parts.join(' ');
		}
		if (el.tagName === 'IMG') return el.getAttribute('alt') || '';
		if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') {
			if (el.id) {
				var lbl = document.querySelector('label[for="' + el.id + '"]');
				if (lbl) return lbl.textContent.trim();
			}
			return el.getAttribute('placeholder') || el.getAttribute('name') || '';
		}
		var title = el.getAttribute('title');
		if (title) return title;
		if (['A','BUTTON','SUMMARY','H1','H2','H3','H4','H5','H6'].indexOf(el.tagName) !== -1) {
			var text = el.textContent.trim();
			if (text.length <= 80) return text;
			return text.substring(0, 77) + '...';
		}
		return '';
	}

	function isInteractable(el) {
		var tag = el.tagName ? el.tagName.toLowerCase() : '';
		if (['a','button','input','select','textarea','summary'].indexOf(tag) !== -1) return true;
		if (el.getAttribute && (el.getAttribute('tabindex') || el.getAttribute('onclick') || el.getAttribute('contenteditable') === 'true')) return true;
		var role = el.getAttribute ? (el.getAttribute('role') || '') : '';
		if (['button','link','textbox','checkbox','radio','combobox','slider','menuitem','tab'].indexOf(role) !== -1) return true;
		return false;
	}

	function isHidden(el) {
		if (!el.getAttribute) return false;
		if (el.getAttribute('aria-hidden') === 'true') return true;
		if (el.getAttribute('hidden') !== null) return true;
		var style = el.style;
		if (style && (style.display === 'none' || style.visibility === 'hidden')) return true;
		return false;
	}

	function walk(node, depth, lines) {
		if (config.maxDepth > 0 && depth > config.maxDepth) return;
		if (node.nodeType !== 1) return;
		if (isHidden(node)) return;

		var role = getRole(node);
		if (config.filterRoles && config.filterRoles.length > 0 && role && config.filterRoles.indexOf(role) === -1) {
			for (var child = node.firstChild; child; child = child.nextSibling) {
				walk(child, depth, lines);
			}
			return;
		}
		if (config.interactableOnly && !isInteractable(node)) {
			for (var child = node.firstChild; child; child = child.nextSibling) {
				walk(child, depth, lines);
			}
			return;
		}
		if (!role) {
			for (var child = node.firstChild; child; child = child.nextSibling) {
				walk(child, depth, lines);
			}
			return;
		}

		refCounter++;
		var ref = 's' + gen + 'e' + refCounter;
		node.setAttribute('data-scout-ref', ref);

		var line = '';
		for (var i = 0; i < depth; i++) line += '  ';
		line += '- ' + role;
		var name = getName(node);
		if (name) line += ' "' + name.replace(/"/g, '\\"') + '"';

		var tag = node.tagName.toLowerCase();
		if (role === 'heading') line += ' level=' + tag.charAt(1);
		if ((tag === 'input' || tag === 'textarea') && node.value) {
			line += ' value="' + node.value.replace(/"/g, '\\"') + '"';
		}
		if (tag === 'input' && (node.type === 'checkbox' || node.type === 'radio')) {
			line += node.checked ? ' checked' : ' unchecked';
		}
		if (node.getAttribute('disabled') !== null) line += ' disabled';
		if (tag === 'a' && node.href) line += ' url="' + node.href + '"';

		line += ' [ref=' + ref + ']';
		lines.push(line);

		for (var child = node.firstChild; child; child = child.nextSibling) {
			walk(child, depth + 1, lines);
		}
	}

	var lines = [];
	refCounter = 0;
	lines.push('- document [ref=s' + gen + 'e0]');
	if (document.body) {
		document.body.setAttribute('data-scout-ref', 's' + gen + 'e0');
		for (var child = document.body.firstChild; child; child = child.nextSibling) {
			walk(child, 1, lines);
		}
	}
	return lines.join('\n');
})`

// elementByRefJS finds an element by its data-scout-ref attribute.
const elementByRefJS = `(function(ref) {
	var el = document.querySelector('[data-scout-ref="' + ref + '"]');
	return el ? true : null;
})`
