from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.alert_rule_severity import AlertRuleSeverity
from ..types import UNSET, Unset

T = TypeVar("T", bound="AlertRule")


@_attrs_define
class AlertRule:
    """
    Attributes:
        id (int | Unset):
        name (str | Unset):
        source (str | Unset): Rule text in the Sorolens rule language. Example: error_rate > 5% for 15m.
        severity (AlertRuleSeverity | Unset):
        contract_id (str | Unset):
        network (str | Unset):
        window (str | Unset):  Example: 15m.
        enabled (bool | Unset):
        created_at (datetime.datetime | Unset):
        updated_at (datetime.datetime | Unset):
    """

    id: int | Unset = UNSET
    name: str | Unset = UNSET
    source: str | Unset = UNSET
    severity: AlertRuleSeverity | Unset = UNSET
    contract_id: str | Unset = UNSET
    network: str | Unset = UNSET
    window: str | Unset = UNSET
    enabled: bool | Unset = UNSET
    created_at: datetime.datetime | Unset = UNSET
    updated_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        name = self.name

        source = self.source

        severity: str | Unset = UNSET
        if not isinstance(self.severity, Unset):
            severity = self.severity.value

        contract_id = self.contract_id

        network = self.network

        window = self.window

        enabled = self.enabled

        created_at: str | Unset = UNSET
        if not isinstance(self.created_at, Unset):
            created_at = self.created_at.isoformat()

        updated_at: str | Unset = UNSET
        if not isinstance(self.updated_at, Unset):
            updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if id is not UNSET:
            field_dict["id"] = id
        if name is not UNSET:
            field_dict["name"] = name
        if source is not UNSET:
            field_dict["source"] = source
        if severity is not UNSET:
            field_dict["severity"] = severity
        if contract_id is not UNSET:
            field_dict["contract_id"] = contract_id
        if network is not UNSET:
            field_dict["network"] = network
        if window is not UNSET:
            field_dict["window"] = window
        if enabled is not UNSET:
            field_dict["enabled"] = enabled
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if updated_at is not UNSET:
            field_dict["updated_at"] = updated_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id", UNSET)

        name = d.pop("name", UNSET)

        source = d.pop("source", UNSET)

        _severity = d.pop("severity", UNSET)
        severity: AlertRuleSeverity | Unset
        if isinstance(_severity, Unset):
            severity = UNSET
        else:
            severity = AlertRuleSeverity(_severity)

        contract_id = d.pop("contract_id", UNSET)

        network = d.pop("network", UNSET)

        window = d.pop("window", UNSET)

        enabled = d.pop("enabled", UNSET)

        _created_at = d.pop("created_at", UNSET)
        created_at: datetime.datetime | Unset
        if isinstance(_created_at, Unset):
            created_at = UNSET
        else:
            created_at = datetime.datetime.fromisoformat(_created_at)

        _updated_at = d.pop("updated_at", UNSET)
        updated_at: datetime.datetime | Unset
        if isinstance(_updated_at, Unset):
            updated_at = UNSET
        else:
            updated_at = datetime.datetime.fromisoformat(_updated_at)

        alert_rule = cls(
            id=id,
            name=name,
            source=source,
            severity=severity,
            contract_id=contract_id,
            network=network,
            window=window,
            enabled=enabled,
            created_at=created_at,
            updated_at=updated_at,
        )

        alert_rule.additional_properties = d
        return alert_rule

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
