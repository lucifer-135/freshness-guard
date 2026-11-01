package heal

const systemPrompt = `You correct outdated statements in an answer with the smallest possible edit and return JSON that matches the schema.
Change only the listed statements: replace each old value with its current value, and keep the wording, language, formatting and everything else exactly as it is.
Do not add explanations, citations or new information.
Return the whole corrected answer in "text".`

const schema = `{"type":"object","additionalProperties":false,"required":["text"],"properties":{"text":{"type":"string"}}}`
