package extract

const systemPrompt = `You extract checkable factual claims from an AI assistant's answer and return JSON that matches the schema.
Extract only factual claims that can change over time or be checked against a source, such as prices, software versions, people in roles, laws, statistics, events and product features.
Do not extract opinions, advice, instructions, questions or creative text.
Copy "quote" from the answer exactly, character for character, and keep it as short as possible while it still contains the claim.
Use category "static" for facts that do not change, such as historical dates and definitions.
"temporal_marker" is a word or phrase in the claim that refers to the present or to a date, such as "currently", "latest" or "as of 2025". Use an empty string when there is none.
"search_query" is a short web search query that would find the current value.
Return at most %d claims, the most time-sensitive first.`

const schema = `{"type":"object","additionalProperties":false,"required":["claims"],"properties":{"claims":{"type":"array","items":{
 "type":"object","additionalProperties":false,
 "required":["quote","entity","attribute","value","category","temporal_marker","search_query"],
 "properties":{"quote":{"type":"string"},"entity":{"type":"string"},"attribute":{"type":"string"},"value":{"type":"string"},
  "category":{"type":"string","enum":["pricing","version","leadership","law","statistic","event","feature","static"]},
  "temporal_marker":{"type":"string"},"search_query":{"type":"string"}}}}}}`
