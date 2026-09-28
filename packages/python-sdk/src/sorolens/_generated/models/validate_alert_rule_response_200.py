from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.rule_diagnostic import RuleDiagnostic


T = TypeVar("T", bound="ValidateAlertRuleResponse200")


@_attrs_define
class ValidateAlertRuleResponse200:
    """
    Attributes:
        valid (bool | Unset):
        normalized (str | Unset):
        metrics (list[str] | Unset):
        window (str | Unset):
        errors (list[RuleDiagnostic] | Unset):
    """

    valid: bool | Unset = UNSET
    normalized: str | Unset = UNSET
    metrics: list[str] | Unset = UNSET
    window: str | Unset = UNSET
    errors: list[RuleDiagnostic] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        valid = self.valid

        normalized = self.normalized

        metrics: list[str] | Unset = UNSET
        if not isinstance(self.metrics, Unset):
            metrics = self.metrics

        window = self.window

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
        if normalized is not UNSET:
            field_dict["normalized"] = normalized
        if metrics is not UNSET:
            field_dict["metrics"] = metrics
        if window is not UNSET:
            field_dict["window"] = window
        if errors is not UNSET:
            field_dict["errors"] = errors

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.rule_diagnostic import RuleDiagnostic

        d = dict(src_dict)
        valid = d.pop("valid", UNSET)

        normalized = d.pop("normalized", UNSET)

        metrics = cast(list[str], d.pop("metrics", UNSET))

        window = d.pop("window", UNSET)

        _errors = d.pop("errors", UNSET)
        errors: list[RuleDiagnostic] | Unset = UNSET
        if _errors is not UNSET:
            errors = []
            for errors_item_data in _errors:
                errors_item = RuleDiagnostic.from_dict(errors_item_data)

                errors.append(errors_item)

        validate_alert_rule_response_200 = cls(
            valid=valid,
            normalized=normalized,
            metrics=metrics,
            window=window,
            errors=errors,
        )

        validate_alert_rule_response_200.additional_properties = d
        return validate_alert_rule_response_200

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
