"""Dated primary-source candidate settings shared with launch-scoped metadata."""
import json
import math
from pathlib import Path

SNAPSHOT_PATH = Path(__file__).resolve().parent.parent / 'internal/tofa/assets/evaluation-candidates.json'


def snapshot():
    return json.loads(SNAPSHOT_PATH.read_text())


def candidate(model):
    data = snapshot()
    if model not in data['models']:
        raise ValueError('candidate_not_shortlisted')
    settings = data['models'][model]
    if not isinstance(settings, dict):
        raise ValueError('candidate_metadata_unresolved')
    if (type(settings.get('context_window')) is not int or settings['context_window'] <= 4096
            or settings.get('responses_api') is not True or settings.get('function_calling') is not True
            or settings.get('input_modalities') not in (['text'], ['text', 'image'])):
        raise ValueError('candidate_metadata_unresolved')
    return settings


def prices(model):
    try:
        data = snapshot()
    except (OSError, ValueError):
        return None
    settings = data['models'].get(model, {})
    rates = [settings.get(key) for key in ('input_price', 'output_price')]
    if any(type(rate) not in (int, float) or not math.isfinite(rate) or rate < 0 for rate in rates):
        return None
    return {'input_usd_per_million': rates[0], 'output_usd_per_million': rates[1],
            'date': data['price_date'], 'source': data['price_source'], 'policy': data['price_policy']}
