// plainSnippet turns a search-result excerpt into plain text. Search results
// often arrive as Markdown (headings, bold, links); shown raw, they read as
// noise. Text that merely looks like HTML is left alone: it is rendered as text.
export function plainSnippet(text: string): string {
  return text
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s*/gm, '')
    .replace(/(\*\*|__)(.+?)\1/g, '$2')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\s+/g, ' ')
    .trim()
}
