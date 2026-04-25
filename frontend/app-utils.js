/*
 * Created by DINKIssTyle on 2026.
 * Copyright (C) 2026 DINKI'ssTyle. All rights reserved.
 */

(function attachDKSTAppUtils(global) {
    function buildSessionFetchOptions(extra = {}) {
        const { headers: extraHeaders = {}, ...rest } = extra || {};
        const headers = {
            ...extraHeaders
        };
        const sessionToken = global.localStorage?.getItem('sessionToken') || '';
        if (sessionToken && !headers.Authorization) {
            headers.Authorization = `Bearer ${sessionToken}`;
        }
        return {
            credentials: 'include',
            cache: 'no-store',
            headers,
            ...rest
        };
    }

    function getMarkdownRenderer() {
        const remarkRenderer = global.remarkMarkdownRenderer;
        if (remarkRenderer?.render) return remarkRenderer;

        if (global.marked?.parse) {
            return {
                name: 'marked',
                render(markdown) {
                    return global.marked.parse(markdown || '');
                }
            };
        }

        return {
            name: 'plain',
            render(markdown) {
                const escaped = String(markdown || '')
                    .replace(/&/g, '&amp;')
                    .replace(/</g, '&lt;')
                    .replace(/>/g, '&gt;');
                return escaped ? `<pre>${escaped}</pre>` : '';
            }
        };
    }

    function escapeHtml(text) {
        return String(text || '')
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function escapeAttr(value) {
        return String(value ?? '')
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function syncHapticsPreference(config) {
        global.DKSTHaptics?.setEnabled(config?.hapticsEnabled !== false);
    }

    function triggerHaptic(config, type) {
        if (config?.hapticsEnabled === false) return;
        global.DKSTHaptics?.trigger(type);
    }

    function renderLooseInlineMarkdown(text) {
        let html = escapeHtml(text);
        html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
        html = html.replace(/\[([^\]\n]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2">$1</a>');
        html = html.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
        html = html.replace(/(^|[^\*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
        return html;
    }

    function renderLooseMarkdownToHtml(markdownText) {
        const lines = String(markdownText || '').split('\n');
        const html = [];
        let paragraph = [];
        let listType = null;

        const flushParagraph = () => {
            if (!paragraph.length) return;
            html.push(`<p>${renderLooseInlineMarkdown(paragraph.join(' '))}</p>`);
            paragraph = [];
        };

        const closeList = () => {
            if (!listType) return;
            html.push(listType === 'ol' ? '</ol>' : '</ul>');
            listType = null;
        };

        const openList = (type) => {
            if (listType === type) return;
            closeList();
            html.push(type === 'ol' ? '<ol>' : '<ul>');
            listType = type;
        };

        for (const rawLine of lines) {
            const line = String(rawLine || '').trim();

            if (!line) {
                flushParagraph();
                closeList();
                continue;
            }

            if (/^---+$/.test(line)) {
                flushParagraph();
                closeList();
                html.push('<hr>');
                continue;
            }

            const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
            if (headingMatch) {
                flushParagraph();
                closeList();
                const level = Math.min(headingMatch[1].length, 6);
                html.push(`<h${level}>${renderLooseInlineMarkdown(headingMatch[2].trim())}</h${level}>`);
                continue;
            }

            const orderedMatch = line.match(/^(\d+)\.\s+(.+)$/);
            if (orderedMatch) {
                flushParagraph();
                openList('ol');
                html.push(`<li>${renderLooseInlineMarkdown(orderedMatch[2].trim())}</li>`);
                continue;
            }

            const unorderedMatch = line.match(/^[-*•●▪■▸▹▻▶▷►]\s+(.+)$/);
            if (unorderedMatch) {
                flushParagraph();
                openList('ul');
                html.push(`<li>${renderLooseInlineMarkdown(unorderedMatch[1].trim())}</li>`);
                continue;
            }

            if (/^[#*•●▪■▸▹▻▶▷►-]+$/.test(line)) {
                continue;
            }

            closeList();
            paragraph.push(line);
        }

        flushParagraph();
        closeList();
        return html.join('');
    }

    function shouldFallbackToLooseMarkdown(host, normalized) {
        if (!host || !normalized.trim()) return false;

        const hasHeadingSyntax = /(^|\n)[ \t]*#{1,6}\s+\S/.test(normalized);
        const hasListSyntax = /(^|\n)[ \t]*(?:[-*•●▪■▸▹▻▶▷►]\s+\S|\d+\.\s+\S)/.test(normalized);
        const text = host.innerText || host.textContent || '';

        if (hasHeadingSyntax && !host.querySelector('h1,h2,h3,h4,h5,h6') && /(^|\n)\s*#\s*\S/.test(text)) {
            return true;
        }

        if (hasListSyntax && !host.querySelector('ul,ol,li') && /(^|\n)\s*(?:[-*•●▪■▸▹▻▶▷►]|\d+\.)\s*\S/.test(text)) {
            return true;
        }

        return false;
    }

    function normalizeInlineMarkdownSpacing(segment) {
        return segment
            .replace(/\*\*[ \t]+([^*\n](?:[^*\n]*?[^*\s\n])?)[ \t]+\*\*/g, '**$1**')
            .replace(/(^|\n)([ \t]*(?:#{1,6}\s|(?:[-*+]\s|\d+\.\s)))(\*\*|__)([^\n]*?)(?=\n|$)/g, (match, prefix, markerPrefix, marker, content) => {
                const trimmed = String(content || '').trim();
                if (!trimmed || trimmed.includes(marker)) return match;
                return `${prefix}${markerPrefix}${marker}${trimmed}${marker}`;
            })
            .replace(/(^|\n)([ \t]*(?:#{1,6}\s|(?:[-*+]\s|\d+\.\s)))(\*|_)([^\n]*?)(?=\n|$)/g, (match, prefix, markerPrefix, marker, content) => {
                const trimmed = String(content || '').trim();
                if (!trimmed || trimmed.startsWith(' ') || trimmed.includes(marker)) return match;
                return `${prefix}${markerPrefix}${marker}${trimmed}${marker}`;
            });
    }

    function normalizeMarkdownOutsideCode(text, transform) {
        const parts = String(text).split(/(```[\s\S]*?```|~~~[\s\S]*?~~~|`[^`\n]+`)/g);
        return parts.map((part, index) => {
            if (index % 2 === 1) return part;
            return transform(part);
        }).join('');
    }

    function protectMathSegments(text) {
        const placeholders = [];
        let index = 0;
        const register = (match) => {
            const token = `@@PROTECTED_MATH_${index++}@@`;
            placeholders.push({ token, value: match });
            return token;
        };

        const protectedText = normalizeMarkdownOutsideCode(text, (segment) =>
            segment.replace(/(\$\$[\s\S]*?\$\$|\\\[[\s\S]*?\\\]|\\\([\s\S]*?\\\)|(?<!\$)\$[^$\n]+\$(?!\$))/g, register)
        );

        return { protectedText, placeholders };
    }

    function countTablePipes(line) {
        return (String(line).match(/\|/g) || []).length;
    }

    function isLikelyTableRow(line) {
        const trimmed = String(line || '').trim();
        if (!trimmed) return false;
        return countTablePipes(trimmed) >= 2;
    }

    function isTableSeparatorRow(line) {
        const trimmed = String(line || '').trim();
        if (!trimmed) return false;
        const normalized = trimmed.replace(/\|/g, '').replace(/:/g, '').replace(/-/g, '').trim();
        return normalized === '' && /-/.test(trimmed);
    }

    function normalizeTableRow(line) {
        let trimmed = String(line || '').trim();
        trimmed = trimmed.replace(/^[-*+]\s+/, '').trim();
        trimmed = trimmed.replace(/^\|\s*/, '').replace(/\s*\|$/, '');
        const cells = trimmed.split('|').map(cell => cell.trim());
        if (cells.length < 2) {
            return String(line || '');
        }
        return `| ${cells.join(' | ')} |`;
    }

    function normalizeTableBlock(block) {
        const rawLines = String(block || '').split('\n').map(line => line.trim()).filter(Boolean);
        if (rawLines.length < 2) {
            return block;
        }

        const normalizedRows = rawLines.map(normalizeTableRow);
        const headerCells = normalizedRows[0]
            .replace(/^\|\s*/, '')
            .replace(/\s*\|$/, '')
            .split('|')
            .map(cell => cell.trim())
            .filter(Boolean);

        if (headerCells.length < 2) {
            return block;
        }

        if (!isTableSeparatorRow(rawLines[1])) {
            const separator = `| ${headerCells.map(() => '---').join(' | ')} |`;
            normalizedRows.splice(1, 0, separator);
        } else {
            normalizedRows[1] = `| ${headerCells.map(() => '---').join(' | ')} |`;
        }

        return normalizedRows.join('\n');
    }

    function canonicalizeTableLikeBlocks(text) {
        const lines = String(text || '').split('\n');
        const result = [];

        for (let i = 0; i < lines.length;) {
            if (!isLikelyTableRow(lines[i])) {
                result.push(lines[i]);
                i += 1;
                continue;
            }

            const block = [];
            let j = i;
            while (j < lines.length && isLikelyTableRow(lines[j])) {
                block.push(lines[j]);
                j += 1;
            }

            if (block.length >= 2) {
                result.push(normalizeTableBlock(block.join('\n')));
            } else {
                result.push(...block);
            }
            i = j;
        }

        return result.join('\n');
    }

    function closeUnbalancedCodeFences(text) {
        const source = String(text || '');
        const backtickFences = source.match(/(^|\n)```/g);
        const hasUnclosedBacktick = backtickFences && backtickFences.length % 2 !== 0;
        const tildeFences = source.match(/(^|\n)~~~/g);
        const hasUnclosedTilde = tildeFences && tildeFences.length % 2 !== 0;

        let result = source;
        if (hasUnclosedBacktick) result += '\n```';
        if (hasUnclosedTilde) result += '\n~~~';
        return result;
    }

    function protectTableSegments(text) {
        const placeholders = [];
        let index = 0;
        const register = (match) => {
            const token = `@@PROTECTED_TABLE_${index++}@@`;
            placeholders.push({ token, value: match });
            return token;
        };

        const protectedText = normalizeMarkdownOutsideCode(text, (segment) =>
            segment.replace(/(^|\n)(\|[^\n]+\|\n\|[-:\s|]+\|(?:\n\s*\n|\n\|[^\n]+\|)+)/g, (match, prefix, tableBlock) => {
                return `${prefix}${register(tableBlock)}`;
            })
        );

        return { protectedText, placeholders };
    }

    function restoreProtectedSegments(text, placeholders) {
        return (placeholders || []).reduce((result, entry) => result.replaceAll(entry.token, entry.value), text);
    }

    function restoreProtectedMathSegments(text, placeholders) {
        return restoreProtectedSegments(text, placeholders);
    }

    function normalizeMarkdownForRender(text) {
        if (!text) return '';

        let normalized = String(text);
        normalized = closeUnbalancedCodeFences(normalized);
        const protectedMath = protectMathSegments(normalized);
        normalized = protectedMath.protectedText;

        normalized = normalized.replace(/[\u200B-\u200D\u2060\uFEFF]/g, '');
        normalized = normalized
            .replace(/(^|\n)([ \t]*)[•●▪■▸▹▻▶▷►]\s+/g, '$1$2- ')
            .replace(/(^|\n)([ \t]*)[◦○◇◆]\s+/g, '$1$2  - ');

        normalized = normalized
            .replace(/\r\n/g, '\n')
            .replace(/\r/g, '\n')
            .replace(/(^|\n)([ \t]*#{1,6})(?=[^\s#])/g, '$1$2 ')
            .replace(/(^|\n)([ \t]*)(\d+)\.(?=\S)/g, '$1$2$3. ')
            .replace(/(^|\n)([ \t]*)(-)(?=[^\s\-])/g, '$1$2$3 ')
            .replace(/(^|\n)([ \t]*)(\*)(?=[^\s*])/g, '$1$2$3 ')
            .replace(/(^|\n)([ \t]*)(\+)(?=[^\s+])/g, '$1$2$3 ')
            .replace(/(^|\n)([ \t]*>)(?=\S)/g, '$1$2 ')
            .replace(/(^|\n)([ \t]*[-*+]\s+\[[ xX]\])(?=\S)/g, '$1$2 ')
            .replace(/(^|\n)[ \t]*[•●▪■▸▹▻▶▷►][ \t]*(?=\n|$)/g, '$1');

        normalized = normalizeMarkdownOutsideCode(normalized, (segment) =>
            normalizeInlineMarkdownSpacing(segment)
                .replace(/(^|\n)([ \t]*[-*+]\s+)\*\*\s+([^*\n]+?)\s+\*\*/g, '$1$2**$3**')
        );

        normalized = normalizeMarkdownOutsideCode(normalized, (segment) =>
            canonicalizeTableLikeBlocks(segment)
        );

        const protectedTables = protectTableSegments(normalized);
        normalized = protectedTables.protectedText;

        normalized = normalizeMarkdownOutsideCode(normalized, (segment) => {
            let result = segment
                .replace(/([^\n#])([ \t]*#{1,6}\s)/g, '$1\n\n$2')
                .replace(/(^|\n)([ \t]*#{1,6}[^\n]*?\S)(?=[ \t]*(?:-(?!-)\s|\+\s|\d+\.\s))/g, '$1$2\n\n');

            result = result.split('\n').map(line => {
                if (/^\s*(?:#{1,6}\s|(?:[-*+]|\d+\.)\s)/.test(line)) return line;
                return line.replace(/([^\s])([ \t]*\*\*[^*\n][^\n]*\*\*)$/g, '$1\n\n$2');
            }).join('\n');

            return result
                .replace(/([.!?;:)\]。！？])([ \t]*(?:[-*+]\s|\d+\.\s))/g, '$1\n\n$2')
                .replace(/([^\n])([ \t]*>)/g, '$1\n\n$2')
                .replace(/([^\n])\n?([ \t]*[-*_]{3,}[ \t]*)(?=\n|$)/g, '$1\n\n$2')
                .replace(/([^\n])([ \t]*\$\$)/g, '$1\n\n$2')
                .replace(/([^\n])\n(#{1,6}\s)/g, '$1\n\n$2')
                .replace(/([^\n])\n((?:[-*+]\s|\d+\.\s))/g, '$1\n\n$2')
                .replace(/\[([^\]]+)\]\s+\((https?:\/\/[^\s)]+)\)/g, '[$1]($2)')
                .replace(/(\|[^\n]+\|)\n(?=\|[-:\s|]+\|)/g, '$1\n')
                .replace(/(\|[^\n]+\|)\n\s*\n(?=\|[-:\s|]+\|)/g, '$1\n')
                .replace(/(\|[-:\s|]+\|)\n\s*\n(?=\|)/g, '$1\n')
                .replace(/(\|[^\n]+\|)\n\s*\n(?=\|[^\n]+\|)/g, '$1\n')
                .replace(/\n{3,}/g, '\n\n');
        });

        normalized = restoreProtectedSegments(normalized, protectedTables.placeholders);
        return restoreProtectedMathSegments(normalized, protectedMath.placeholders);
    }

    function sanitizeRenderedMarkdownHtml(html) {
        const rawHtml = String(html || '');
        if (!rawHtml.trim()) return '';

        const svgTags = [
            'svg', 'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polyline',
            'polygon', 'text', 'tspan', 'defs', 'marker', 'pattern',
            'lineargradient', 'radialgradient', 'stop', 'clippath', 'mask',
            'title', 'desc', 'style', 'use', 'foreignobject'
        ];
        const svgAttrs = [
            'id', 'role', 'aria-hidden', 'aria-label', 'aria-labelledby',
            'aria-describedby', 'aria-roledescription', 'focusable', 'tabindex', 'xmlns', 'xmlns:xlink',
            'viewbox', 'preserveaspectratio', 'width', 'height', 'x', 'y', 'x1',
            'y1', 'x2', 'y2', 'cx', 'cy', 'r', 'rx', 'ry', 'd', 'points',
            'transform', 'fill', 'stroke', 'stroke-width', 'stroke-linecap',
            'stroke-linejoin', 'stroke-dasharray', 'stroke-dashoffset',
            'opacity', 'fill-opacity', 'stroke-opacity', 'marker-start',
            'marker-mid', 'marker-end', 'orient', 'refx', 'refy', 'markerwidth',
            'markerheight', 'text-anchor', 'dominant-baseline', 'font-family',
            'font-size', 'font-weight', 'font-style', 'dy', 'dx', 'offset',
            'stop-color', 'stop-opacity', 'clip-path', 'clip-rule', 'fill-rule',
            'mask', 'patternunits', 'patterncontentunits', 'href', 'xlink:href',
            'style', 'alignment-baseline', 'baseline-shift', 'color', 'cursor',
            'display', 'paint-order', 'version', 'xml:space'
        ];

        if (global.DOMPurify?.sanitize) {
            return global.DOMPurify.sanitize(rawHtml, {
                ADD_TAGS: svgTags,
                ADD_ATTR: svgAttrs
            });
        }

        const template = global.document.createElement('template');
        template.innerHTML = rawHtml;

        const allowedTags = new Set([
            'a', 'abbr', 'b', 'blockquote', 'br', 'code', 'del', 'details', 'div', 'em',
            'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr', 'i', 'img', 'kbd', 'li', 'mark',
            'ol', 'p', 'pre', 'q', 'rp', 'rt', 'ruby', 's', 'samp', 'small', 'span',
            'strong', 'sub', 'summary', 'sup', 'table', 'tbody', 'td', 'th', 'thead',
            'tr', 'ul',
            ...svgTags
        ]);
        const globalAllowedAttrs = new Set([
            'class', 'title', 'aria-hidden', 'aria-label', 'aria-labelledby',
            'aria-describedby', 'aria-roledescription', 'id', 'role'
        ]);
        const tagAllowedAttrs = {
            a: new Set(['href', 'target', 'rel']),
            img: new Set(['src', 'alt', 'title']),
            code: new Set(['class']),
            span: new Set(['class', 'style', 'xmlns']),
            div: new Set(['class', 'style', 'xmlns']),
            th: new Set(['colspan', 'rowspan']),
            td: new Set(['colspan', 'rowspan'])
        };
        svgTags.forEach((tag) => {
            tagAllowedAttrs[tag] = new Set(svgAttrs);
        });

        const sanitizeUrl = (value) => {
            const normalized = String(value || '').trim();
            if (!normalized) return '';
            if (normalized.startsWith('#') || normalized.startsWith('/') || normalized.startsWith('./') || normalized.startsWith('../')) {
                return normalized;
            }
            try {
                const parsed = new URL(normalized, global.location.origin);
                const protocol = parsed.protocol.toLowerCase();
                if (protocol === 'http:' || protocol === 'https:' || protocol === 'mailto:') {
                    return parsed.href;
                }
            } catch (_) {
                return '';
            }
            return '';
        };

        const sanitizeSvgReference = (value) => {
            const normalized = String(value || '').trim();
            if (!normalized) return '';
            if (/^url\(\s*#[A-Za-z][\w:.-]*\s*\)$/.test(normalized)) return normalized;
            if (/^#[A-Za-z][\w:.-]*$/.test(normalized)) return normalized;
            return sanitizeUrl(normalized);
        };

        const sanitizeStyleValue = (value) => {
            const css = String(value || '').trim();
            if (!css) return '';
            if (/(?:javascript\s*:|expression\s*\(|@import|<\s*\/?\s*script)/i.test(css)) {
                return '';
            }
            if (/url\s*\(\s*(['"]?)(?!#)/i.test(css)) {
                return '';
            }
            return css;
        };

        const walker = global.document.createTreeWalker(template.content, NodeFilter.SHOW_ELEMENT);
        const nodes = [];
        while (walker.nextNode()) {
            nodes.push(walker.currentNode);
        }

        nodes.forEach((node) => {
            const tagName = node.tagName.toLowerCase();
            if (!allowedTags.has(tagName)) {
                node.replaceWith(global.document.createTextNode(node.textContent || ''));
                return;
            }

            const isInsideSvg = tagName === 'svg' || !!node.closest('svg');
            if (tagName === 'style' && !isInsideSvg) {
                node.remove();
                return;
            }

            Array.from(node.attributes).forEach((attr) => {
                const attrName = attr.name.toLowerCase();
                const allowedForTag = tagAllowedAttrs[tagName];
                const isAllowed = attrName.startsWith('data-')
                    || globalAllowedAttrs.has(attrName)
                    || allowedForTag?.has(attrName);
                if (!isAllowed || attrName.startsWith('on')) {
                    node.removeAttribute(attr.name);
                    return;
                }

                if (attrName === 'href' || attrName === 'src' || attrName === 'xlink:href') {
                    const sanitized = tagName === 'use' || attrName === 'xlink:href'
                        ? sanitizeSvgReference(attr.value)
                        : sanitizeUrl(attr.value);
                    if (!sanitized) {
                        node.removeAttribute(attr.name);
                        return;
                    }
                    node.setAttribute(attr.name, sanitized);
                }

                if (attrName === 'style') {
                    if (!isInsideSvg) {
                        node.removeAttribute(attr.name);
                        return;
                    }
                    const sanitized = sanitizeStyleValue(attr.value);
                    if (!sanitized) {
                        node.removeAttribute(attr.name);
                        return;
                    }
                    node.setAttribute(attr.name, sanitized);
                }

                if (attrName === 'marker-start' || attrName === 'marker-mid' || attrName === 'marker-end'
                    || attrName === 'clip-path' || attrName === 'mask' || attrName === 'filter') {
                    const sanitized = sanitizeSvgReference(attr.value);
                    if (!sanitized) {
                        node.removeAttribute(attr.name);
                        return;
                    }
                    node.setAttribute(attr.name, sanitized);
                }
            });

            if (tagName === 'a') {
                node.setAttribute('target', '_blank');
                node.setAttribute('rel', 'noopener noreferrer');
            }

            if (tagName === 'style') {
                const sanitized = sanitizeStyleValue(node.textContent || '');
                if (!sanitized) {
                    node.remove();
                    return;
                }
                node.textContent = sanitized;
            }
        });

        return template.innerHTML;
    }

    global.DKSTAppUtils = {
        buildSessionFetchOptions,
        escapeAttr,
        escapeHtml,
        getMarkdownRenderer,
        normalizeMarkdownForRender,
        renderLooseMarkdownToHtml,
        sanitizeRenderedMarkdownHtml,
        shouldFallbackToLooseMarkdown,
        syncHapticsPreference,
        triggerHaptic
    };
})(window);
