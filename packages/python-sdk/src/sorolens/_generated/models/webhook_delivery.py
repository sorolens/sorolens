from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.webhook_delivery_status import WebhookDeliveryStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.webhook_delivery_signed_payload_type_0 import (
        WebhookDeliverySignedPayloadType0,
    )


T = TypeVar("T", bound="WebhookDelivery")


@_attrs_define
class WebhookDelivery:
    """One webhook delivery attempt for an alert subscription (issue

    Attributes:
        id (str):
        subscription_id (str):
        attempt_number (int):
        status (WebhookDeliveryStatus):
        created_at (datetime.datetime):
        updated_at (datetime.datetime):
        signed_payload (None | Unset | WebhookDeliverySignedPayloadType0): The JSONB body sent (or to be sent) to the
            webhook URL.
        response_status (int | None | Unset): HTTP status code returned by the destination.
        response_body (None | str | Unset): Response body from the destination (truncated at 4 KiB).
        last_error (None | str | Unset): Last transport-level error (network timeout, DNS failure, etc.).
        next_retry_at (datetime.datetime | None | Unset): When the delivery will next be retried; null if no retry
            scheduled.
    """

    id: str
    subscription_id: str
    attempt_number: int
    status: WebhookDeliveryStatus
    created_at: datetime.datetime
    updated_at: datetime.datetime
    signed_payload: None | Unset | WebhookDeliverySignedPayloadType0 = UNSET
    response_status: int | None | Unset = UNSET
    response_body: None | str | Unset = UNSET
    last_error: None | str | Unset = UNSET
    next_retry_at: datetime.datetime | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.webhook_delivery_signed_payload_type_0 import (
            WebhookDeliverySignedPayloadType0,
        )

        id = self.id

        subscription_id = self.subscription_id

        attempt_number = self.attempt_number

        status = self.status.value

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        signed_payload: dict[str, Any] | None | Unset
        if isinstance(self.signed_payload, Unset):
            signed_payload = UNSET
        elif isinstance(self.signed_payload, WebhookDeliverySignedPayloadType0):
            signed_payload = self.signed_payload.to_dict()
        else:
            signed_payload = self.signed_payload

        response_status: int | None | Unset
        if isinstance(self.response_status, Unset):
            response_status = UNSET
        else:
            response_status = self.response_status

        response_body: None | str | Unset
        if isinstance(self.response_body, Unset):
            response_body = UNSET
        else:
            response_body = self.response_body

        last_error: None | str | Unset
        if isinstance(self.last_error, Unset):
            last_error = UNSET
        else:
            last_error = self.last_error

        next_retry_at: None | str | Unset
        if isinstance(self.next_retry_at, Unset):
            next_retry_at = UNSET
        elif isinstance(self.next_retry_at, datetime.datetime):
            next_retry_at = self.next_retry_at.isoformat()
        else:
            next_retry_at = self.next_retry_at

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "subscription_id": subscription_id,
                "attempt_number": attempt_number,
                "status": status,
                "created_at": created_at,
                "updated_at": updated_at,
            }
        )
        if signed_payload is not UNSET:
            field_dict["signed_payload"] = signed_payload
        if response_status is not UNSET:
            field_dict["response_status"] = response_status
        if response_body is not UNSET:
            field_dict["response_body"] = response_body
        if last_error is not UNSET:
            field_dict["last_error"] = last_error
        if next_retry_at is not UNSET:
            field_dict["next_retry_at"] = next_retry_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.webhook_delivery_signed_payload_type_0 import (
            WebhookDeliverySignedPayloadType0,
        )

        d = dict(src_dict)
        id = d.pop("id")

        subscription_id = d.pop("subscription_id")

        attempt_number = d.pop("attempt_number")

        status = WebhookDeliveryStatus(d.pop("status"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        def _parse_signed_payload(
            data: object,
        ) -> None | Unset | WebhookDeliverySignedPayloadType0:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                signed_payload_type_0 = WebhookDeliverySignedPayloadType0.from_dict(
                    data
                )

                return signed_payload_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | WebhookDeliverySignedPayloadType0, data)

        signed_payload = _parse_signed_payload(d.pop("signed_payload", UNSET))

        def _parse_response_status(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        response_status = _parse_response_status(d.pop("response_status", UNSET))

        def _parse_response_body(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        response_body = _parse_response_body(d.pop("response_body", UNSET))

        def _parse_last_error(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        last_error = _parse_last_error(d.pop("last_error", UNSET))

        def _parse_next_retry_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                next_retry_at_type_0 = datetime.datetime.fromisoformat(data)

                return next_retry_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        next_retry_at = _parse_next_retry_at(d.pop("next_retry_at", UNSET))

        webhook_delivery = cls(
            id=id,
            subscription_id=subscription_id,
            attempt_number=attempt_number,
            status=status,
            created_at=created_at,
            updated_at=updated_at,
            signed_payload=signed_payload,
            response_status=response_status,
            response_body=response_body,
            last_error=last_error,
            next_retry_at=next_retry_at,
        )

        webhook_delivery.additional_properties = d
        return webhook_delivery

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
