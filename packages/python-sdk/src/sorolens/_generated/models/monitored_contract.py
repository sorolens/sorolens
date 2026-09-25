from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="MonitoredContract")


@_attrs_define
class MonitoredContract:
    """
    Attributes:
        contract_id (str):
        network (str):
        name (str):
        owner (str):
        status (str):
        last_check (datetime.datetime | None):
        check_interval (int):
        registered_at (datetime.datetime):
        updated_at (datetime.datetime):
    """

    contract_id: str
    network: str
    name: str
    owner: str
    status: str
    last_check: datetime.datetime | None
    check_interval: int
    registered_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        network = self.network

        name = self.name

        owner = self.owner

        status = self.status

        last_check: None | str
        if isinstance(self.last_check, datetime.datetime):
            last_check = self.last_check.isoformat()
        else:
            last_check = self.last_check

        check_interval = self.check_interval

        registered_at = self.registered_at.isoformat()

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "network": network,
                "name": name,
                "owner": owner,
                "status": status,
                "last_check": last_check,
                "check_interval": check_interval,
                "registered_at": registered_at,
                "updated_at": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        name = d.pop("name")

        owner = d.pop("owner")

        status = d.pop("status")

        def _parse_last_check(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                last_check_type_0 = datetime.datetime.fromisoformat(data)

                return last_check_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        last_check = _parse_last_check(d.pop("last_check"))

        check_interval = d.pop("check_interval")

        registered_at = datetime.datetime.fromisoformat(d.pop("registered_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        monitored_contract = cls(
            contract_id=contract_id,
            network=network,
            name=name,
            owner=owner,
            status=status,
            last_check=last_check,
            check_interval=check_interval,
            registered_at=registered_at,
            updated_at=updated_at,
        )

        monitored_contract.additional_properties = d
        return monitored_contract

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
