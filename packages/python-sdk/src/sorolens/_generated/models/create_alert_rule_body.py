from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_alert_rule_body_severity import CreateAlertRuleBodySeverity
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateAlertRuleBody")


@_attrs_define
class CreateAlertRuleBody:
    """
    Attributes:
        name (str):
        source (str):  Example: fee_per_invocation > 0.5 XLM for 5m.
        severity (CreateAlertRuleBodySeverity | Unset):  Default: CreateAlertRuleBodySeverity.WARNING.
        contract_id (str | Unset):
        network (str | Unset):
    """

    name: str
    source: str
    severity: CreateAlertRuleBodySeverity | Unset = CreateAlertRuleBodySeverity.WARNING
    contract_id: str | Unset = UNSET
    network: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        source = self.source

        severity: str | Unset = UNSET
        if not isinstance(self.severity, Unset):
            severity = self.severity.value

        contract_id = self.contract_id

        network = self.network

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "source": source,
            }
        )
        if severity is not UNSET:
            field_dict["severity"] = severity
        if contract_id is not UNSET:
            field_dict["contract_id"] = contract_id
        if network is not UNSET:
            field_dict["network"] = network

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        source = d.pop("source")

        _severity = d.pop("severity", UNSET)
        severity: CreateAlertRuleBodySeverity | Unset
        if isinstance(_severity, Unset):
            severity = UNSET
        else:
            severity = CreateAlertRuleBodySeverity(_severity)

        contract_id = d.pop("contract_id", UNSET)

        network = d.pop("network", UNSET)

        create_alert_rule_body = cls(
            name=name,
            source=source,
            severity=severity,
            contract_id=contract_id,
            network=network,
        )

        create_alert_rule_body.additional_properties = d
        return create_alert_rule_body

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
