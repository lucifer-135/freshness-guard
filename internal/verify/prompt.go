package verify

const systemPrompt = `You decide whether factual claims are still current, using only the evidence provided. Today's date is given in the input. Return one result for each claim.
FRESH means the evidence confirms that the claim is true today.
STALE means the claim was true at some time but has been superseded.
WRONG means the claim was never true. Use WRONG only when the evidence shows that the claimed value never held; when the evidence shows only that a different value holds now, use STALE.
UNVERIFIABLE means the evidence is not enough to decide.
For FRESH, STALE and WRONG, "current_value" is the value that holds today according to the evidence, or an empty string when the evidence does not show it.
When sources disagree, the newest dated authoritative source wins. Official sites, government and education sites, and major publishers are authoritative.
Do not use knowledge that is not in the evidence.
"as_of" is the date (YYYY-MM-DD) from which "current_value" is known to hold, or an empty string.
"supporting_evidence_ids" lists the refs of the evidence that supports your verdict. "contradicting_evidence_ids" lists the refs of the evidence that contradicts the value in the claim.
"timeline" lists the known values of the fact in date order, each with the ref of its source.
"confidence" is a number from 0 to 1. "rationale" is one or two sentences.`

const schema = `{"type":"object","additionalProperties":false,"required":["results"],"properties":{"results":{"type":"array","items":{
 "type":"object","additionalProperties":false,
 "required":["claim_id","verdict","current_value","as_of","supporting_evidence_ids","contradicting_evidence_ids","timeline","confidence","rationale"],
 "properties":{"claim_id":{"type":"string"},"verdict":{"type":"string","enum":["FRESH","STALE","WRONG","UNVERIFIABLE"]},
  "current_value":{"type":"string"},"as_of":{"type":"string"},
  "supporting_evidence_ids":{"type":"array","items":{"type":"string"}},"contradicting_evidence_ids":{"type":"array","items":{"type":"string"}},
  "timeline":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["value","valid_from","source"],
   "properties":{"value":{"type":"string"},"valid_from":{"type":"string"},"source":{"type":"string"}}}},
  "confidence":{"type":"number"},"rationale":{"type":"string"}}}}}}`
