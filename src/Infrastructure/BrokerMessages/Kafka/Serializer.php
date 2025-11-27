<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Infrastructure\BrokerMessages\Kafka;

use Symfony\Component\Messenger\Envelope;
use Symfony\Component\Messenger\Transport\Serialization\SerializerInterface;

final class Serializer implements SerializerInterface
{
    /**
     * @param array<string, mixed> $encodedEnvelope
     */
    public function decode(array $encodedEnvelope): Envelope
    {
        return new Envelope((object) $encodedEnvelope);
    }

    /**
     * @return array<string, mixed>
     *
     * @throws \JsonException
     */
    public function encode(Envelope $envelope): array
    {
        $event = $envelope->getMessage();
        $eventCode = $event instanceof EncodeInterface ? $event->getEventCode() : 'none';

        $body = json_encode((array) $event + ['code' => $eventCode], JSON_THROW_ON_ERROR);

        return [
            'key' => $event instanceof EncodeInterface ? $event->getKey() : hash('sha256', $body),
            'headers' => [],
            'body' => $body,
        ];
    }
}
