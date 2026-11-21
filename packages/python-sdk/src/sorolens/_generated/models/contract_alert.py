from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.contract_alert_severity import ContractAlertSeverity
from ..models.contract_alert_state import ContractAlertState
from ..types import UNSET, Unset

T = TypeVar("T", bound="ContractAlert")


@_attrs_define
class ContractAlert:
    """
    Attributes:
        contract_id (str):
        severity (ContractAlertSeverity):
        message (str):
        ledger (int):
        tx_hash (str):
        timestamp (datetime.datetime):
        id (int | Unset):
        state (ContractAlertState | Unset):
        acknowledged_at (datetime.datetime | None | Unset):
        acknowledged_by (str | Unset):
        snoozed_until (datetime.datetime | None | Unset):
        snoozed_at (datetime.datetime | None | Unset):
        snoozed_by (str | Unset):
        resolved_at (datetime.datetime | None | Unset):
        resolved_by (str | Unset):
        maintenance_window_id (int | None | Unset):
    """

    contract_id: str
    severity: ContractAlertSeverity
    message: str
    ledger: int
    tx_hash: str
    timestamp: datetime.datetime
    id: int | Unset = UNSET
    state: ContractAlertState | Unset = UNSET
    acknowledged_at: datetime.datetime | None | Unset = UNSET
    acknowledged_by: str | Unset = UNSET
    snoozed_until: datetime.datetime | None | Unset = UNSET
    snoozed_at: datetime.datetime | None | Unset = UNSET
    snoozed_by: str | Unset = UNSET
    resolved_at: datetime.datetime | None | Unset = UNSET
    resolved_by: str | Unset = UNSET
    maintenance_window_id: int | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        severity = self.severity.value

        message = self.message

        ledger = self.ledger

        tx_hash = self.tx_hash

        timestamp = self.timestamp.isoformat()

        id = self.id

        state: str | Unset = UNSET
        if not isinstance(self.state, Unset):
            state = self.state.value

        acknowledged_at: None | str | Unset
        if isinstance(self.acknowledged_at, Unset):
            acknowledged_at = UNSET
        elif isinstance(self.acknowledged_at, datetime.datetime):
            acknowledged_at = self.acknowledged_at.isoformat()
        else:
            acknowledged_at = self.acknowledged_at

        acknowledged_by = self.acknowledged_by

        snoozed_until: None | str | Unset
        if isinstance(self.snoozed_until, Unset):
            snoozed_until = UNSET
        elif isinstance(self.snoozed_until, datetime.datetime):
            snoozed_until = self.snoozed_until.isoformat()
        else:
            snoozed_until = self.snoozed_until

        snoozed_at: None | str | Unset
        if isinstance(self.snoozed_at, Unset):
            snoozed_at = UNSET
        elif isinstance(self.snoozed_at, datetime.datetime):
            snoozed_at = self.snoozed_at.isoformat()
        else:
            snoozed_at = self.snoozed_at

        snoozed_by = self.snoozed_by

        resolved_at: None | str | Unset
        if isinstance(self.resolved_at, Unset):
            resolved_at = UNSET
        elif isinstance(self.resolved_at, datetime.datetime):
            resolved_at = self.resolved_at.isoformat()
        else:
            resolved_at = self.resolved_at

        resolved_by = self.resolved_by

        maintenance_window_id: int | None | Unset
        if isinstance(self.maintenance_window_id, Unset):
            maintenance_window_id = UNSET
        else:
            maintenance_window_id = self.maintenance_window_id

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "severity": severity,
                "message": message,
                "ledger": ledger,
                "tx_hash": tx_hash,
                "timestamp": timestamp,
            }
        )
        if id is not UNSET:
            field_dict["id"] = id
        if state is not UNSET:
            field_dict["state"] = state
        if acknowledged_at is not UNSET:
            field_dict["acknowledged_at"] = acknowledged_at
        if acknowledged_by is not UNSET:
            field_dict["acknowledged_by"] = acknowledged_by
        if snoozed_until is not UNSET:
            field_dict["snoozed_until"] = snoozed_until
        if snoozed_at is not UNSET:
            field_dict["snoozed_at"] = snoozed_at
        if snoozed_by is not UNSET:
            field_dict["snoozed_by"] = snoozed_by
        if resolved_at is not UNSET:
            field_dict["resolved_at"] = resolved_at
        if resolved_by is not UNSET:
            field_dict["resolved_by"] = resolved_by
        if maintenance_window_id is not UNSET:
            field_dict["maintenance_window_id"] = maintenance_window_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        severity = ContractAlertSeverity(d.pop("severity"))

        message = d.pop("message")

        ledger = d.pop("ledger")

        tx_hash = d.pop("tx_hash")

        timestamp = datetime.datetime.fromisoformat(d.pop("timestamp"))

        id = d.pop("id", UNSET)

        _state = d.pop("state", UNSET)
        state: ContractAlertState | Unset
        if isinstance(_state, Unset):
            state = UNSET
        else:
            state = ContractAlertState(_state)

        def _parse_acknowledged_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                acknowledged_at_type_0 = datetime.datetime.fromisoformat(data)

                return acknowledged_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        acknowledged_at = _parse_acknowledged_at(d.pop("acknowledged_at", UNSET))

        acknowledged_by = d.pop("acknowledged_by", UNSET)

        def _parse_snoozed_until(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                snoozed_until_type_0 = datetime.datetime.fromisoformat(data)

                return snoozed_until_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        snoozed_until = _parse_snoozed_until(d.pop("snoozed_until", UNSET))

        def _parse_snoozed_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                snoozed_at_type_0 = datetime.datetime.fromisoformat(data)

                return snoozed_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        snoozed_at = _parse_snoozed_at(d.pop("snoozed_at", UNSET))

        snoozed_by = d.pop("snoozed_by", UNSET)

        def _parse_resolved_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                resolved_at_type_0 = datetime.datetime.fromisoformat(data)

                return resolved_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        resolved_at = _parse_resolved_at(d.pop("resolved_at", UNSET))

        resolved_by = d.pop("resolved_by", UNSET)

        def _parse_maintenance_window_id(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        maintenance_window_id = _parse_maintenance_window_id(
            d.pop("maintenance_window_id", UNSET)
        )

        contract_alert = cls(
            contract_id=contract_id,
            severity=severity,
            message=message,
            ledger=ledger,
            tx_hash=tx_hash,
            timestamp=timestamp,
            id=id,
            state=state,
            acknowledged_at=acknowledged_at,
            acknowledged_by=acknowledged_by,
            snoozed_until=snoozed_until,
            snoozed_at=snoozed_at,
            snoozed_by=snoozed_by,
            resolved_at=resolved_at,
            resolved_by=resolved_by,
            maintenance_window_id=maintenance_window_id,
        )

        contract_alert.additional_properties = d
        return contract_alert

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
