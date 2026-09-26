from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.alert_group_severity import AlertGroupSeverity

T = TypeVar("T", bound="AlertGroup")


@_attrs_define
class AlertGroup:
    """
    Attributes:
        id (int):
        group_key (str):
        contract_id (str):
        severity (AlertGroupSeverity):
        rule (str):
        count (int):
        dedupe_window_secs (int):
        first_seen (datetime.datetime):
        last_seen (datetime.datetime):
        last_message (str):
        backfill_eligible (bool):
    """

    id: int
    group_key: str
    contract_id: str
    severity: AlertGroupSeverity
    rule: str
    count: int
    dedupe_window_secs: int
    first_seen: datetime.datetime
    last_seen: datetime.datetime
    last_message: str
    backfill_eligible: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        group_key = self.group_key

        contract_id = self.contract_id

        severity = self.severity.value

        rule = self.rule

        count = self.count

        dedupe_window_secs = self.dedupe_window_secs

        first_seen = self.first_seen.isoformat()

        last_seen = self.last_seen.isoformat()

        last_message = self.last_message

        backfill_eligible = self.backfill_eligible

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "group_key": group_key,
                "contract_id": contract_id,
                "severity": severity,
                "rule": rule,
                "count": count,
                "dedupe_window_secs": dedupe_window_secs,
                "first_seen": first_seen,
                "last_seen": last_seen,
                "last_message": last_message,
                "backfill_eligible": backfill_eligible,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        group_key = d.pop("group_key")

        contract_id = d.pop("contract_id")

        severity = AlertGroupSeverity(d.pop("severity"))

        rule = d.pop("rule")

        count = d.pop("count")

        dedupe_window_secs = d.pop("dedupe_window_secs")

        first_seen = datetime.datetime.fromisoformat(d.pop("first_seen"))

        last_seen = datetime.datetime.fromisoformat(d.pop("last_seen"))

        last_message = d.pop("last_message")

        backfill_eligible = d.pop("backfill_eligible")

        alert_group = cls(
            id=id,
            group_key=group_key,
            contract_id=contract_id,
            severity=severity,
            rule=rule,
            count=count,
            dedupe_window_secs=dedupe_window_secs,
            first_seen=first_seen,
            last_seen=last_seen,
            last_message=last_message,
            backfill_eligible=backfill_eligible,
        )

        alert_group.additional_properties = d
        return alert_group

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
