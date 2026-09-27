from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.preview_alert_rule_response_200_points_item import (
        PreviewAlertRuleResponse200PointsItem,
    )
    from ..models.rule_diagnostic import RuleDiagnostic


T = TypeVar("T", bound="PreviewAlertRuleResponse200")


@_attrs_define
class PreviewAlertRuleResponse200:
    """
    Attributes:
        valid (bool | Unset):
        fired (bool | Unset):
        op (str | Unset):
        value (float | None | Unset):
        threshold (float | None | Unset):
        reason (str | Unset):
        window (str | Unset):
        evaluated_at (datetime.datetime | Unset):
        points (list[PreviewAlertRuleResponse200PointsItem] | Unset):
        errors (list[RuleDiagnostic] | Unset):
    """

    valid: bool | Unset = UNSET
    fired: bool | Unset = UNSET
    op: str | Unset = UNSET
    value: float | None | Unset = UNSET
    threshold: float | None | Unset = UNSET
    reason: str | Unset = UNSET
    window: str | Unset = UNSET
    evaluated_at: datetime.datetime | Unset = UNSET
    points: list[PreviewAlertRuleResponse200PointsItem] | Unset = UNSET
    errors: list[RuleDiagnostic] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        valid = self.valid

        fired = self.fired

        op = self.op

        value: float | None | Unset
        if isinstance(self.value, Unset):
            value = UNSET
        else:
            value = self.value

        threshold: float | None | Unset
        if isinstance(self.threshold, Unset):
            threshold = UNSET
        else:
            threshold = self.threshold

        reason = self.reason

        window = self.window

        evaluated_at: str | Unset = UNSET
        if not isinstance(self.evaluated_at, Unset):
            evaluated_at = self.evaluated_at.isoformat()

        points: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.points, Unset):
            points = []
            for points_item_data in self.points:
                points_item = points_item_data.to_dict()
                points.append(points_item)

        errors: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.errors, Unset):
            errors = []
            for errors_item_data in self.errors:
                errors_item = errors_item_data.to_dict()
                errors.append(errors_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if valid is not UNSET:
            field_dict["valid"] = valid
        if fired is not UNSET:
            field_dict["fired"] = fired
        if op is not UNSET:
            field_dict["op"] = op
        if value is not UNSET:
            field_dict["value"] = value
        if threshold is not UNSET:
            field_dict["threshold"] = threshold
        if reason is not UNSET:
            field_dict["reason"] = reason
        if window is not UNSET:
            field_dict["window"] = window
        if evaluated_at is not UNSET:
            field_dict["evaluated_at"] = evaluated_at
        if points is not UNSET:
            field_dict["points"] = points
        if errors is not UNSET:
            field_dict["errors"] = errors

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.preview_alert_rule_response_200_points_item import (
            PreviewAlertRuleResponse200PointsItem,
        )
        from ..models.rule_diagnostic import RuleDiagnostic

        d = dict(src_dict)
        valid = d.pop("valid", UNSET)

        fired = d.pop("fired", UNSET)

        op = d.pop("op", UNSET)

        def _parse_value(data: object) -> float | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(float | None | Unset, data)

        value = _parse_value(d.pop("value", UNSET))

        def _parse_threshold(data: object) -> float | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(float | None | Unset, data)

        threshold = _parse_threshold(d.pop("threshold", UNSET))

        reason = d.pop("reason", UNSET)

        window = d.pop("window", UNSET)

        _evaluated_at = d.pop("evaluated_at", UNSET)
        evaluated_at: datetime.datetime | Unset
        if isinstance(_evaluated_at, Unset):
            evaluated_at = UNSET
        else:
            evaluated_at = datetime.datetime.fromisoformat(_evaluated_at)

        _points = d.pop("points", UNSET)
        points: list[PreviewAlertRuleResponse200PointsItem] | Unset = UNSET
        if _points is not UNSET:
            points = []
            for points_item_data in _points:
                points_item = PreviewAlertRuleResponse200PointsItem.from_dict(
                    points_item_data
                )

                points.append(points_item)

        _errors = d.pop("errors", UNSET)
        errors: list[RuleDiagnostic] | Unset = UNSET
        if _errors is not UNSET:
            errors = []
            for errors_item_data in _errors:
                errors_item = RuleDiagnostic.from_dict(errors_item_data)

                errors.append(errors_item)

        preview_alert_rule_response_200 = cls(
            valid=valid,
            fired=fired,
            op=op,
            value=value,
            threshold=threshold,
            reason=reason,
            window=window,
            evaluated_at=evaluated_at,
            points=points,
            errors=errors,
        )

        preview_alert_rule_response_200.additional_properties = d
        return preview_alert_rule_response_200

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
