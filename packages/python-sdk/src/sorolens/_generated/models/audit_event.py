from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="AuditEvent")


@_attrs_define
class AuditEvent:
    """
    Attributes:
        id (int):
        action (str): Method and route pattern, e.g. "POST /api/v1/contracts".
        status (int): HTTP response status returned for the request.
        at (datetime.datetime):
        actor (str | Unset): Authenticated actor (API key label or role identity); empty for anonymous requests.
        resource_type (str | Unset):
        resource_id (str | Unset):
        ip (str | Unset):
        user_agent (str | Unset):
        request_body_hash (str | Unset): SHA-256 hex of the request body; never the body itself.
    """

    id: int
    action: str
    status: int
    at: datetime.datetime
    actor: str | Unset = UNSET
    resource_type: str | Unset = UNSET
    resource_id: str | Unset = UNSET
    ip: str | Unset = UNSET
    user_agent: str | Unset = UNSET
    request_body_hash: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        action = self.action

        status = self.status

        at = self.at.isoformat()

        actor = self.actor

        resource_type = self.resource_type

        resource_id = self.resource_id

        ip = self.ip

        user_agent = self.user_agent

        request_body_hash = self.request_body_hash

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "action": action,
                "status": status,
                "at": at,
            }
        )
        if actor is not UNSET:
            field_dict["actor"] = actor
        if resource_type is not UNSET:
            field_dict["resource_type"] = resource_type
        if resource_id is not UNSET:
            field_dict["resource_id"] = resource_id
        if ip is not UNSET:
            field_dict["ip"] = ip
        if user_agent is not UNSET:
            field_dict["user_agent"] = user_agent
        if request_body_hash is not UNSET:
            field_dict["request_body_hash"] = request_body_hash

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        action = d.pop("action")

        status = d.pop("status")

        at = datetime.datetime.fromisoformat(d.pop("at"))

        actor = d.pop("actor", UNSET)

        resource_type = d.pop("resource_type", UNSET)

        resource_id = d.pop("resource_id", UNSET)

        ip = d.pop("ip", UNSET)

        user_agent = d.pop("user_agent", UNSET)

        request_body_hash = d.pop("request_body_hash", UNSET)

        audit_event = cls(
            id=id,
            action=action,
            status=status,
            at=at,
            actor=actor,
            resource_type=resource_type,
            resource_id=resource_id,
            ip=ip,
            user_agent=user_agent,
            request_body_hash=request_body_hash,
        )

        audit_event.additional_properties = d
        return audit_event

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
